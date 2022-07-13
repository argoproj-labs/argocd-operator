package libs

import (
	"context"
	pb "smtplistener/internal/compressorclient/pb"
)

// Encode and Decode Compress Request

func EncodeGRPCCompressRequest(_ context.Context, r interface{}) (interface{}, error) {
	req := r.(CompressRequest)
	return &pb.CompressRequest{
		CompressAlgo: req.CompressAlgo,
		Data:         req.Data,
	}, nil
}

func DecodeGRPCCompressRequest(ctx context.Context, r interface{}) (interface{}, error) {
	req := r.(*pb.CompressRequest)
	return pb.CompressRequest{
		CompressAlgo: req.CompressAlgo, Data: req.Data,
	}, nil
}

// Encode and Decode Compress Response
func EncodeGRPCCompressResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(CompressResponse)
	return &pb.CompressResponse{
		StatusMessage:  resp.StatusMessage,
		Err:            resp.Err,
		CompressedData: resp.CompressedData,
	}, nil
}

func DecodeGRPCCompressResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(*pb.CompressResponse)
	return pb.CompressResponse{
		StatusMessage:  resp.StatusMessage,
		Err:            resp.Err,
		CompressedData: resp.CompressedData,
	}, nil
}

// Encode and Decode DeCompress Request

func EncodeGRPCDeCompressRequest(_ context.Context, r interface{}) (interface{}, error) {
	req := r.(DeCompressRequest)
	return &pb.DeCompressRequest{
		CompressAlgo:   req.CompressAlgo,
		CompressedData: req.CompressedData,
	}, nil
}

func DecodeGRPCDeCompressRequest(ctx context.Context, r interface{}) (interface{}, error) {
	req := r.(*pb.DeCompressRequest)
	return pb.DeCompressRequest{
		CompressAlgo: req.CompressAlgo, CompressedData: req.CompressedData,
	}, nil
}

// Encode and Decode DECompress Response

func EncodeGRPCDeCompressResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(DeCompressResponse)
	return &pb.DeCompressResponse{
		StatusMessage: resp.StatusMessage,
		Err:           resp.Err,
		Data:          resp.Data,
	}, nil
}

func DecodeGRPCDeCompressResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(*pb.DeCompressResponse)
	return pb.DeCompressResponse{
		StatusMessage: resp.StatusMessage,
		Err:           resp.Err,
		Data:          resp.Data,
	}, nil
}

// Encode and Decode Health Request
func EncodeGRPCHealthRequest(_ context.Context, r interface{}) (interface{}, error) {
	req := r.(pb.CompressHealthRequest)
	return &req, nil
}

func DecodeGRPCHealthRequest(ctx context.Context, r interface{}) (interface{}, error) {
	req := r.(*pb.CompressHealthRequest)
	return req, nil
}

// Encode and Decode Health Response

func EncodeGRPCHealthResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(pb.CompressHealthResponse)
	return &resp, nil
}

func DecodeGRPCHealthResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(*pb.CompressHealthResponse)
	return resp, nil
}

// Encode and Decode MTACompress Request

func EncodeGRPCMTACompressRequest(_ context.Context, r interface{}) (interface{}, error) {
	req := r.(pb.MTACompressRequest)
	return &req, nil
}

func DecodeGRPCMTACompressRequest(ctx context.Context, r interface{}) (interface{}, error) {
	req := r.(*pb.MTACompressRequest)
	return req, nil
}

// Encode and Decode MTACompress Response
func EncodeGRPCMTACompressResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(pb.MTACompressResponse)
	return &resp, nil
}

func DecodeGRPCMTACompressResponse(_ context.Context, r interface{}) (interface{}, error) {
	resp := r.(*pb.MTACompressResponse)
	return resp, nil
}
