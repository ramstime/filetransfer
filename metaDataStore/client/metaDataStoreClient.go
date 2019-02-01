package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	mds "metaDataStore/MetaDataStore"

	"github.com/goinggo/tracelog"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const chunkSize = 64 * 1024 // 64 KiB

//ClientGRPC will hold var required for connection
type ClientGRPC struct {
	conn      *grpc.ClientConn
	client    mds.MetaDataStoreClient
	chunkSize int
}

//Stats will show time
type Stats struct {
	StartedAt  time.Time
	FinishedAt time.Time
}

//ClientGRPCConfig will hold configs
type ClientGRPCConfig struct {
	Address         string
	ChunkSize       int
	RootCertificate string
	Compress        bool
}

//NewClientGRPC will create grpc connection and return client
func NewClientGRPC(cfg ClientGRPCConfig) (c ClientGRPC, err error) {
	tracelog.Started("MetaDataStoreClient", "NewClientGRPC")
	var (
		grpcOpts  = []grpc.DialOption{}
		grpcCreds credentials.TransportCredentials
	)

	if cfg.Address == "" {
		err = errors.Errorf("address must be specified")
		return
	}

	if cfg.Compress {
		grpcOpts = append(grpcOpts,
			grpc.WithDefaultCallOptions(grpc.UseCompressor("tar")))
	}

	if cfg.RootCertificate != "" {
		grpcCreds, err = credentials.NewClientTLSFromFile(cfg.RootCertificate, "localhost")
		if err != nil {
			err = errors.Wrapf(err,
				"failed to create grpc tls client via root-cert %s",
				cfg.RootCertificate)
			return
		}

		grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(grpcCreds))
	} else {
		grpcOpts = append(grpcOpts, grpc.WithInsecure())
	}

	switch {
	case cfg.ChunkSize == 0:
		err = errors.Errorf("ChunkSize must be specified")
		return
	case cfg.ChunkSize > (1 << 22):
		err = errors.Errorf("ChunkSize must be < than 4MB")
		return
	default:
		c.chunkSize = cfg.ChunkSize
	}
	tracelog.Info("MetaDataStoreClient", "NewClientGRPC", "connecting to :%s", cfg.Address)
	c.conn, err = grpc.Dial(cfg.Address, grpcOpts...)
	//c.conn, err = grpc.Dial(cfg.Address, grpc.WithInsecure())
	if err != nil {
		err = errors.Wrapf(err,
			"failed to start grpc connection with address %s",
			cfg.Address)
		return
	}
	tracelog.Info("MetaDataStoreClient", "NewClientGRPC", "client Dail success")
	c.client = mds.NewMetaDataStoreClient(c.conn)

	return
}

//UploadFile func will upload the tar file to server
func (c *ClientGRPC) UploadFile(ctx context.Context, f string) (stats Stats, err error) {
	tracelog.Started("MetaDataStoreClient", "UploadFile")
	var (
		writing = true
		buf     []byte
		n       int
		file    *os.File
		resp    *mds.UploadResponse
	)
	tracelog.Info("MetaDataStoreClient", "UploadFile", "file:%s", f)
	file, err = os.Open(f)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to open file %s",
			f)
		return
	}
	defer file.Close()
	tracelog.Info("MetaDataStoreClient", "UploadFile", "calling onboard")
	stream, err := c.client.Upload(ctx)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to create upload stream for file %s",
			f)
		return
	}
	defer stream.CloseSend()

	stats.StartedAt = time.Now()
	buf = make([]byte, c.chunkSize)
	for writing {
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

		metaDataInfo :=
			&mds.MetaDataInfo{
				SourceType:      "project1",
				Version:         "major/18.0|minor/10.0",
				MetadataType:    mds.MetadataType_type1,
				MetadataSubtype: mds.MetadataSubType_subtype1,
			}

		err = stream.Send(&mds.UploadRequest{
			MetadataInfo: metaDataInfo,
			MetadataTar:  buf[:n],
		})
		if err != nil {
			err = errors.Wrapf(err,
				"failed to send chunk via stream")
			return
		}
	}

	stats.FinishedAt = time.Now()

	resp, err = stream.CloseAndRecv()
	if err != nil {
		err = errors.Wrapf(err,
			"failed to receive upstream status response")
		return
	}

	if resp.Status.Statuscode != mds.Statuscode_ok {
		err = errors.Errorf(
			"upload failed - msg: %s",
			resp.Status.DetailedErrorResponse)
		return
	}
	tracelog.Info("MetaDataStoreClient", "Onboard", "success")

	return
}

//DownloadFile will get requested file
func (c *ClientGRPC) DownloadFile(ctx context.Context) (stats Stats, err error) {

	tracelog.Started("MetaDataStoreClient", "ReadFile")

	tracelog.Info("MetaDataStoreClient", "ReadFile", "enter")

	metaDataInfo :=
		&mds.MetaDataInfo{
			SourceType:      "project1",
			Version:         "major/18.0|minor/10.0",
			MetadataType:    mds.MetadataType_type1,
			MetadataSubtype: mds.MetadataSubType_subtype1,
		}

	var req *mds.DownloadRequest = &mds.DownloadRequest{
		MetadataInfo: metaDataInfo,
	}
	stats.StartedAt = time.Now()
	stream, err := c.client.Download(ctx, req)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to read stream for file ")
		return
	}
	var metadataTar []byte
	var resp *mds.DownloadResponse
	fmt.Printf("\n")
	for {
		//tracelog.Info("", "", "#")
		fmt.Printf("#")
		resp, err = stream.Recv()
		if err != nil {
			if err == io.EOF {
				tracelog.Info("MetaDataStoreClient", "Onboard", "EOF reached")
				goto END
			}

			err = errors.Wrapf(err,
				"failed unexpectadely while reading chunks from stream")
			return
		}
		metadataTar = append(metadataTar, resp.MetadataTar...)
		//request.MetadataTar.Buffer.Write(req.MetadataTar)
	}

END:
	var status mds.Status
	status.Statuscode = mds.Statuscode_ok
	// once the transmission finished, send the
	// confirmation if nothign went wrong
	//err = stream.CloseAndRecv()
	// ...
	stats.FinishedAt = time.Now()
	err = ioutil.WriteFile("./client_recieved_metadata.tar", metadataTar, 0644)
	if err != nil {
		tracelog.Errorf(err, "MetaDataStoreClient", "Read", "write failed")
	}

	return
}

//Close will close the grpc connection
func (c *ClientGRPC) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

//Configuration having server ip and port
type Configuration struct {
	ServerIP   string
	ServerPort string
	FilePath   string
}

// readConfig will read ip and port of the event receiver server for communication
func (conf *Configuration) readConfig() (err error) {

	tracelog.Started("Configuration", "readConfig")
	tracelog.Info("Configuration", "readConfig", "Reading Config file")
	var file *os.File
	file, err = os.Open("./config.json")
	if err != nil {
		tracelog.Info("Configuraton", "readConfig", "Couldn't open the file")
		tracelog.CompletedError(err, "Configuration", "readConfig")
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(file)

	decodeResult := decoder.Decode(conf)
	if decodeResult != nil {
		tracelog.Info("Configuration", "readConfig", "File content decoding failed")
	}

	tracelog.Info("Configuration", "readConfig", "ServerIp: %s ", conf.ServerIP)
	tracelog.Info("Configuration", "readConfig", "ServerPort:  %s", conf.ServerPort)
	tracelog.Info("Configuration", "readConfig", "ServerPort: %s", conf.FilePath)

	tracelog.Completed("Configuration", "readConfig")
	return

}

//read configuration file
func (conf *Configuration) readConfigurationfromFile() (dest string, err error) {

	// TODO Implement logic to get the server IP information ENVOY from console
	// Entire implemention read config will change
	// TODO Need to implement the error handling while implementing with ENVOY

	tracelog.Started("Configurarion", "readConfig")
	err = conf.readConfig()
	if err != nil {
		tracelog.Errorf(err, "MetaDataStoreClient", "Configuration", "Failed to read config file")
		return
	}

	dest = conf.ServerIP + ":" + conf.ServerPort
	tracelog.Info("Configuration", "readConfigurationfromFile", "Connecting to %s", dest)
	tracelog.Completed("Configuration", "readConfigurationfromFile")

	return
}

func main() {
	tracelog.Start(tracelog.LevelTrace)
	//var conn *grpc.ClientConn

	config := Configuration{}
	_, err := config.readConfigurationfromFile()
	if err != nil {
		tracelog.Errorf(err, "MetaDataStoreClient", "main", "Failed to read config file")
		return
	}

	var (
		chunkSize       = chunkSize
		address         = config.ServerIP + ":" + config.ServerPort //"localhost:8888"
		file            = config.FilePath                           //"metadata.tar"
		rootCertificate = ""                                        //c.String("root-certificate")
		compress        = false
	)

	grpcClient, err := NewClientGRPC(ClientGRPCConfig{
		Address:         address,
		RootCertificate: rootCertificate,
		Compress:        compress,
		ChunkSize:       chunkSize,
	})
	if err != nil {

		tracelog.Errorf(err, "MetaDataStoreClient", "main", "Failed to connect grpc Server")
		return
	}
	defer grpcClient.Close()
	stat, err := grpcClient.UploadFile(context.Background(), file)
	if err != nil {

		tracelog.Errorf(err, "MetaDataStoreClient", "main", "Failed to uploadFile to grpc Server")
		return
	}
	tracelog.Info("MetaDataStoreClient", "main", "Upload file finished. time taken: %f\n", stat.FinishedAt.Sub(stat.StartedAt).Seconds())
	stats, errs := grpcClient.DownloadFile(context.Background())
	if errs != nil {

		tracelog.Errorf(errs, "MetaDataStoreClient", "main", "Failed to uploadFile to grpc Server")
		return
	}
	tracelog.Info("MetaDataStoreClient", "main", "reading file finished. time taken: %f\n", stats.FinishedAt.Sub(stats.StartedAt).Seconds())

}
