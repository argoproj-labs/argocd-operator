package libs

import (
	"context"
	"errors"
	"smtplistener/internal/compressorclient/pb"

	"github.com/go-kit/kit/endpoint"
)

type DeCompressRequest struct {
	CompressAlgo   string
	CompressedData string
}

type DeCompressResponse struct {
	StatusMessage bool   `json:"success"`
	Data          string `json:"data,omitempty"`
	Err           string `json:"err,omitempty"`
}

type CompressRequest struct {
	CompressAlgo string
	Data         string
}

type CompressResponse struct {
	StatusMessage  bool   `json:"success"`
	CompressedData string `json:"data,omitempty"`
	Err            string `json:"err,omitempty"`
}

type Endpoints struct {
	CompressionEndpoint   endpoint.Endpoint
	DeCompressionEndpoint endpoint.Endpoint
	HealthCheck           endpoint.Endpoint
	MTACompression        endpoint.Endpoint
}

func (e Endpoints) Compress(ctx context.Context, compressAlgo string, data string) (bool, string, error) {
	req := CompressRequest{
		CompressAlgo: compressAlgo,
		Data:         data,
	}
	resp, err := e.CompressionEndpoint(ctx, req)
	if err != nil {
		return false, "", err
	}
	compressResp := resp.(pb.CompressResponse)
	if compressResp.Err != "" {
		return compressResp.StatusMessage, "", errors.New(compressResp.Err)
	}
	return compressResp.StatusMessage, compressResp.CompressedData, nil
}

func (e Endpoints) DeCompress(ctx context.Context, compressAlgo string, compressedData string) (bool, string, error) {
	req := DeCompressRequest{
		CompressAlgo:   compressAlgo,
		CompressedData: compressedData,
	}
	resp, err := e.DeCompressionEndpoint(ctx, req)
	if err != nil {
		return false, "", err
	}
	deCompressResp := resp.(pb.DeCompressResponse)
	if deCompressResp.Err != "" {
		return deCompressResp.StatusMessage, "", errors.New(deCompressResp.Err)
	}
	return deCompressResp.StatusMessage, deCompressResp.Data, nil
}

func (e Endpoints) Health(ctx context.Context, healthRequest pb.CompressHealthRequest) (pb.CompressHealthResponse, error) {
	resp, _ := e.HealthCheck(ctx, healthRequest)
	return *resp.(*pb.CompressHealthResponse), nil
}

func (e Endpoints) MTACompress(ctx context.Context, mtaCompressRequest pb.MTACompressRequest) (pb.MTACompressResponse, error) {
	resp, err := e.MTACompression(ctx, mtaCompressRequest)
	if err != nil {
		return pb.MTACompressResponse{MtaSuccessMessage: false}, err
	}
	return *resp.(*pb.MTACompressResponse), err
}
