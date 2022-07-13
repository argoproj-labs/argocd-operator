package util

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
)

func SrvDnsResolver(fqdn string) (string, uint16, error) {
	_, addr, err := net.LookupSRV("", "", fqdn)
	if err != nil {
		log.Println("error", err)
		return "", 0, err
	}
	return addr[0].Target, addr[0].Port, nil
}

func GetFqdnName(serviceName string, namespace string, environment string, portvalues ...interface{}) (string, error) {
	if len(portvalues) > 0 && len(portvalues) < 3 {
		return fmt.Sprintf("%s.%s.%s.%s.svc.cluster.%s", portvalues[0], portvalues[1], serviceName, namespace, environment), nil
	}
	return fmt.Sprintf("%s.%s.svc.cluster.%s", serviceName, namespace, environment), nil
}

func WriteToDisk(filepath, message string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	_, err = f.WriteString(message)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

func ZlibCompression(data string) (string, error) {
	var b bytes.Buffer
	zw, err := zlib.NewWriterLevel(&b, zlib.BestSpeed)
	if err != nil {
		return "", err
	}
	if _, err := zw.Write([]byte(data)); err != nil {
		return "", err
	}
	if err := zw.Flush(); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	str := base64.StdEncoding.EncodeToString(b.Bytes())
	return str, nil
}

func GetConfig() string {
	return "conf.json"
}

func Btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
