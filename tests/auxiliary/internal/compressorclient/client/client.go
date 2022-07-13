package client

import (
	"fmt"
	"smtplistener/internal/compressorclient/libs"
	retry "smtplistener/internal/compressorclient/libs/retry"
	pb "smtplistener/internal/compressorclient/pb"
	object "smtplistener/internal/processorobject"
	"smtplistener/internal/util"
	"time"

	log "github.com/flashmob/go-guerrilla/log"
	grpctransport "github.com/go-kit/kit/transport/grpc"
	grpcpool "github.com/processout/grpc-go-pool"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/keepalive"
)

func ClientPool(address string, initial, capacity int, idleTimeout time.Duration, grpcClientSettings object.GrpcClientSettingsConfig) (*grpcpool.Pool, error) {
	//grpcClientSettings := grpcClientSettingsConfig.GrpcClientSettings[0]
	var factory grpcpool.Factory
	var kacp = keepalive.ClientParameters{
		// send pings every 10 seconds if there is no activity
		Time: time.Duration(grpcClientSettings.Time) * time.Second,
		// wait 1 second for ping ack before considering the connection dead
		Timeout: time.Duration(grpcClientSettings.Timeout) * time.Second,
		// send pings even without active streams
		PermitWithoutStream: grpcClientSettings.PermitWithoutStream,
	}
	logs, _ := log.GetLogger(log.OutputStderr.String(), log.InfoLevel.String())
	factory = func() (*grpc.ClientConn, error) {
		//	addr := &address
		conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBalancerName(roundrobin.Name), grpc.WithKeepaliveParams(kacp))
		if err != nil {
			logs.Error("Failed To Create Compressor Connection Pool msg=%v", err.Error())
			return nil, err
		}
		logs.Infof("New client connection to compressor service created")
		return conn, err
	}

	pool, err := grpcpool.New(factory, initial, capacity, idleTimeout)
	if err != nil {
		return nil, err
	}
	return pool, err
}

func Client(serviceName, namespace, environment, host string, port int64, grpcClientSettings object.GrpcClientSettingsConfig, portValues ...interface{}) *grpc.ClientConn {
	var kacp = keepalive.ClientParameters{
		// send pings every 10 seconds if there is no activity
		Time: time.Duration(grpcClientSettings.Time) * time.Second,
		// wait 1 second for ping ack before considering the connection dead
		Timeout: time.Duration(grpcClientSettings.Timeout) * time.Second,
		// send pings even without active streams
		PermitWithoutStream: grpcClientSettings.PermitWithoutStream,
	}
	fqdn, _ := util.GetFqdnName(serviceName, namespace, environment, portValues...)

	var fqdnPort uint16
	err := retry.Do(
		func() error {
			var err error
			_, fqdnPort, err = util.SrvDnsResolver(fqdn)
			return err
		},
		retry.DelayType(func(n uint, config *retry.Config) time.Duration {
			return 5 * time.Second
		}),
	)

	if err != nil {
		logrus.WithFields(logrus.Fields{"error": err}).Error("Error while resolving DNS+SRV records")
	}
	var grpcAddr string
	if err != nil {
		grpcAddr = fmt.Sprintf("%s:%d", host, port)
	} else {
		grpcAddr = fmt.Sprintf("dns:///%s:%d", fqdn, fqdnPort)
	}

	var connection *grpc.ClientConn
	err = retry.Do(
		func() error {
			var err error
			connection, err = grpc.Dial(grpcAddr, grpc.WithInsecure(), grpc.WithBalancerName(roundrobin.Name), grpc.WithKeepaliveParams(kacp))
			return err
		},
		retry.DelayType(func(n uint, config *retry.Config) time.Duration {
			return 5 * time.Second
		}),
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fqdn":  fqdn,
			"error": err,
		}).Fatalf("Error while connecting to %s Service", serviceName)
	}
	return connection
}

func New(conn *grpc.ClientConn) libs.Service {
	var compressEndpoint = grpctransport.NewClient(
		conn, "pb.Compressor", "Compress",
		libs.EncodeGRPCCompressRequest,
		libs.DecodeGRPCCompressResponse,
		pb.CompressResponse{},
	).Endpoint()

	var deCompressEndpoint = grpctransport.NewClient(
		conn, "pb.Compressor", "DeCompress",
		libs.EncodeGRPCDeCompressRequest,
		libs.DecodeGRPCDeCompressResponse,
		pb.DeCompressResponse{},
	).Endpoint()

	var health = grpctransport.NewClient(
		conn, "pb.Compressor", "Health",
		libs.EncodeGRPCHealthRequest,
		libs.DecodeGRPCHealthResponse,
		pb.CompressHealthResponse{},
	).Endpoint()

	var mtaCompressEndpoint = grpctransport.NewClient(
		conn, "pb.Compressor", "MTACompress",
		libs.EncodeGRPCMTACompressRequest,
		libs.DecodeGRPCMTACompressResponse,
		pb.MTACompressResponse{},
	).Endpoint()

	return libs.Endpoints{
		CompressionEndpoint:   compressEndpoint,
		DeCompressionEndpoint: deCompressEndpoint,
		HealthCheck:           health,
		MTACompression:        mtaCompressEndpoint,
	}
}

func FailOverNew(conn *grpc.ClientConn) libs.Service {
	var mtaCompressEndpoint = grpctransport.NewClient(
		conn, "pb.Router", "MTACompress",
		libs.EncodeGRPCMTACompressRequest,
		libs.DecodeGRPCMTACompressResponse,
		pb.MTACompressResponse{},
	).Endpoint()

	return libs.Endpoints{
		MTACompression: mtaCompressEndpoint,
	}
}
