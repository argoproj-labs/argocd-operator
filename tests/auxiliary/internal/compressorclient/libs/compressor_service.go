package libs

import (
    "context"
    "smtplistener/internal/compressorclient/pb"
)

type Service interface {
    Compress(ctx context.Context, compressAlgo string, data string) (bool, string, error)
    DeCompress(ctx context.Context, compressAlgo string, compressedData string) (bool, string, error)
    MTACompress(ctx context.Context, compressRequest pb.MTACompressRequest) (pb.MTACompressResponse, error)
    Health(ctx context.Context, healthRequest pb.CompressHealthRequest) (pb.CompressHealthResponse, error)
}

type CompressService struct {
}
