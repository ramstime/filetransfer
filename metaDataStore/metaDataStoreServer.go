package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	mds "metaDataStore/MetaDataStore"

	"github.com/goinggo/tracelog"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const chunkSize = 64 * 1024 // 64 KiB

//MetaDataStoreService struct
type MetaDataStoreService struct {
	//Structure having grpc client connections
	name      string
	chunkSize int
}

//Stats will show time
type Stats struct {
	StartedAt  time.Time
	FinishedAt time.Time
}

//Upload will upload files to metadata store
func (s *MetaDataStoreService) Upload(stream mds.MetaDataStore_UploadServer) (err error) {

	request := &mds.UploadRequest{}
	tracelog.Started("MetaDataStoreServer", "Onboard")
	tracelog.Info("MetaDataStoreServer", "Onboard", "enter")
	fmt.Printf("\n")
	for {
		//tracelog.Info("MetaDataStoreServer", "Onboard", "#")
		fmt.Printf("#")
		req, errs := stream.Recv()
		if errs != nil {
			if errs == io.EOF {
				tracelog.Info("MetaDataStoreServer", "Onboard", "EOF reached")
				goto END
			}

			err = errors.Wrapf(err,
				"failed unexpectadely while reading chunks from stream")
			return
		}
		request.MetadataTar = append(request.MetadataTar, req.MetadataTar...)
		//request.MetadataTar.Buffer.Write(req.MetadataTar)
		request.MetadataInfo = req.MetadataInfo
	}

END:
	var status mds.Status
	status.Statuscode = mds.Statuscode_ok
	// once the transmission finished, send the
	// confirmation if nothign went wrong
	err = stream.SendAndClose(&mds.UploadResponse{
		Status: &status,
	})
	// ...
	var f string
	if request.MetadataInfo != nil {
		f, err = s.extractFileName(request.MetadataInfo)
		err := ioutil.WriteFile(f, request.MetadataTar, 0644)
		if err != nil {
			tracelog.Errorf(err, "MetaDataStoreServer", "Onboard", "write failed for file:%s", f)
		}
		tracelog.Info("MetaDataStoreServer", "Onboard", "wrote success :%s", f)

	} else {

		err := ioutil.WriteFile("./recieved_metata.tar", request.MetadataTar, 0644)
		if err != nil {
			tracelog.Errorf(err, "MetaDataStoreServer", "Onboard", "write failed for file:recieved_metata")
		}
		tracelog.Info("MetaDataStoreServer", "Onboard", "wrote success: ./recieved_metata.tar")
	}

	return nil
}

func (s *MetaDataStoreService) extractFileName(metadataInfo *mds.MetaDataInfo) (file string, err error) {
	tracelog.Started("MetaDataStoreServer", "extractFileName")
	tracelog.Info("MetaDataStoreServer", "extractFileName", "enter")

	if metadataInfo == nil {
		err = errors.New("metadatainfor is nil")
		tracelog.Errorf(err, "MetaDataStoreServer", "extractFileName", "metadataInfo is nil")
		return
	}
	var dirPath string
	if metadataInfo.Version != "" {
		tracelog.Info("MetaDataStoreServer", "extractFileName", "Version: %s", metadataInfo.Version)
		versions := strings.Split(metadataInfo.Version, "|")
		tracelog.Info("MetaDataStoreServer", "extractFileName", "versions: %s", versions)
		majorVersion := strings.Split(versions[0], "/")
		tracelog.Info("MetaDataStoreServer", "extractFileName", "majorVersion: %s", majorVersion)
		minorVersion := strings.Split(versions[1], "/")
		tracelog.Info("MetaDataStoreServer", "extractFileName", "minorVersion: %s", minorVersion)
		var fileName string
		if mds.MetadataType_type1 == metadataInfo.MetadataType {
			fileName = "metadata.tar"
		}
		dirPath = "./store/" + metadataInfo.SourceType + "/" + majorVersion[1] + "/" + minorVersion[1] + "/"
		file = "./store/" + metadataInfo.SourceType + "/" + majorVersion[1] + "/" + minorVersion[1] + "/" + fileName
	}
	pathErr := os.MkdirAll(dirPath, 0777)
	if pathErr != nil {
		tracelog.Errorf(pathErr, "MetaDataStoreServer", "extractFileName", "failed to create dir:%s", pathErr)
	}
	tracelog.Info("MetaDataStoreServer", "extractFileName", "file: %s", file)
	return
}

//Download method will get all metadata requested
func (s *MetaDataStoreService) Download(readReq *mds.DownloadRequest, serv mds.MetaDataStore_DownloadServer) (err error) {
	tracelog.Started("MetaDataStoreClient", "Read")
	var (
		writing = true
		buf     []byte
		n       int
		file    *os.File
		f       string
	)
	f, err = s.extractFileName(readReq.MetadataInfo)
	tracelog.Info("MetaDataStoreServer", "Read", "file:%s", f)
	file, err = os.Open(f)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to open file %s",
			f)
		tracelog.Errorf(err, "MetaDataStoreServer", "Read", "failed to open")
		return
	}
	defer file.Close()
	tracelog.Info("MetaDataStoreServer", "Read", "Read file ")

	//defer serv.CloseSend()
	var stats Stats
	stats.StartedAt = time.Now()
	buf = make([]byte, s.chunkSize)
	for writing {
		//tracelog.Info("", "", "#")
		fmt.Printf("#")
		n, err = file.Read(buf)
		if err != nil {
			if err == io.EOF {
				writing = false
				err = nil
				continue
			}

			err = errors.Wrapf(err,
				"errored while copying from file to buf")
			return
		}

		resp := &mds.DownloadResponse{Status: &mds.Status{Statuscode: mds.Statuscode_ok}}
		resp.MetadataTar = buf[:n]

		err = serv.Send(resp)

		if err != nil {
			err = errors.Wrapf(err,
				"failed to send chunk via stream")
			return
		}
	}

	stats.FinishedAt = time.Now()

	tracelog.Info("MetaDataStoreServer", "Read", "finished read")

	return
}

//Delete the metadata tar files
func (s *MetaDataStoreService) Delete(ctx context.Context, dreq *mds.DeleteRequest) (resp *mds.DeleteResponse, err error) {
	return
}

func main() {
	tracelog.Start(tracelog.LevelTrace)
	//lis, err := net.Listen("tcp", "10.136.201.155:8888")
	lis, err := net.Listen("tcp", "localhost:8888")
	if err != nil {

		tracelog.Errorf(err, "MetaDataStoreServer", "main", "failed to listen on : localhost:8888 ")
		return
	}

	g := grpc.NewServer()
	mds.RegisterMetaDataStoreServer(g, &MetaDataStoreService{chunkSize: chunkSize})

	tracelog.Info("MetaDataStoreServer", "main", "Serving on localhost:8888")
	log.Fatalln(g.Serve(lis))
}
