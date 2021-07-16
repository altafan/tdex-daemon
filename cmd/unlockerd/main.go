package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pbunlocker "github.com/tdex-network/tdex-daemon/api-spec/protobuf/gen/walletunlocker"
)

type providerFactory func() (provider, error)
type providerMap map[string]providerFactory

func (t providerMap) keys() []string {
	k := make([]string, 0, len(t))
	for key, _ := range t {
		k = append(k, key)
	}
	return k
}

type provider interface {
	Password() ([]byte, error)
	TLSCertificate() ([]byte, error)
}

const (
	rpcAddressKey = "rpc-address"
	providerKey   = "provider"
	intervalKey   = "interval"
)

var (
	// maxMsgRecvSize is the largest message our client will receive. We
	// set this to 200MiB atm.
	maxMsgRecvSize = grpc.MaxCallRecvMsgSize(1 * 1024 * 1024 * 200)

	// supportedProviders is the list of currently supported providers from where
	// to source daemon's password and possibly TLS certificate.
	supportedProviders = providerMap{
		"file": NewFileProvider,
	}

	// flags' default values
	defaultRPCAddress = "localhost:9000"
	defaultProvider   = "file"
	defaultInterval   = 5

	// flags
	rpcAddressFlag = pflag.String(
		rpcAddressKey,
		defaultRPCAddress,
		"specify a daemon's RPC address different from the default one",
	)
	providerFlag = pflag.String(
		providerKey,
		defaultProvider,
		"the provider from where to source password and possibly TLS certificate",
	)
	intervalFlag = pflag.Int(
		intervalKey,
		defaultInterval,
		"the interval in seconds to poll the daemon's IsReady RPC",
	)
)

func init() {
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	if err := validate(); err != nil {
		log.Fatalf("invalid flag: %s", err)
	}
}

func validate() error {
	rpcAddress := viper.GetString(rpcAddressKey)
	if rpcAddress == "" {
		return fmt.Errorf("%s must not be null", rpcAddressKey)
	}
	parts := strings.Split(rpcAddress, ":")
	if len(parts) != 2 {
		return fmt.Errorf("%s must be a valid address in the form host:port", rpcAddressKey)
	}
	if parts[0] != "" && parts[0] != "localhost" {
		if net.ParseIP(parts[0]) == nil {
			return fmt.Errorf("%s host is invalid", rpcAddressKey)
		}
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil || port <= 1024 {
		return fmt.Errorf("%s port is invalid", rpcAddressKey)
	}

	provider := viper.GetString(providerKey)
	if provider == "" {
		return fmt.Errorf("%s must not be null", providerKey)
	}
	if _, ok := supportedProviders[provider]; !ok {
		return fmt.Errorf(
			"unknown provider '%s', must be one of [%s]",
			provider, strings.Join(supportedProviders.keys(), ","),
		)
	}

	interval := viper.GetInt(intervalKey)
	if interval <= 0 {
		return fmt.Errorf("%s must be a positive number", intervalKey)
	}

	return nil
}

func main() {
	rpcAddress := viper.GetString(rpcAddressKey)
	providerType := viper.GetString(providerKey)
	interval := time.Duration(viper.GetInt(intervalKey)) * time.Second

	prov, err := supportedProviders[providerType]()
	if err != nil {
		log.Fatal(err)
	}

	password, err := prov.Password()
	if err != nil {
		log.Fatalf("error while sourcing password: %s", err)
	}
	tlsCertificate, err := prov.TLSCertificate()
	if err != nil {
		log.Fatalf("error while sourcing TLS certificate: %s", err)
	}

	client, cleanup, err := getUnlockerClient(rpcAddress, tlsCertificate)
	if err != nil {
		log.Fatalf("error while setting up gRPC client: %s", err)
	}
	defer cleanup()

	status, err := getWalletStatus(client)
	if err != nil {
		log.Fatalf("error while retrieving wallet status: %s", err)
	}
	if !status.Initialized {
		log.Info("waiting for the wallet to be initialized")
	}
	for !status.Initialized {
		time.Sleep(interval)

		status, err := getWalletStatus(client)
		if err != nil {
			log.Fatalf("error while retrieving wallet status: %s", err)
		}
		if status.Initialized {
			break
		}
	}
	log.Info("wallet initialized")

	if status.Unlocked {
		log.Info("wallet is already unlocked. Nothing left to do")
		return
	}

	log.Info("Attempting to unlock it with provided password...")

	if _, err := client.UnlockWallet(context.Background(), &pbunlocker.UnlockWalletRequest{
		WalletPassword: password,
	}); err != nil {
		log.Fatalf("error while unlocking wallet: %s", err)
	}

	log.Info("wallet unlocked successfully!")
}

func getWalletStatus(client pbunlocker.WalletUnlockerClient) (*pbunlocker.IsReadyReply, error) {
	ctx := context.Background()
	return client.IsReady(ctx, &pbunlocker.IsReadyRequest{})
}

func pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		u, err := user.Current()
		if err == nil {
			homeDir = u.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

func getUnlockerClient(
	rpcAddress string, tlsCertificate []byte,
) (pbunlocker.WalletUnlockerClient, func(), error) {
	conn, err := getClientConn(rpcAddress, tlsCertificate)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = conn.Close() }

	return pbunlocker.NewWalletUnlockerClient(conn), cleanup, nil
}

func getClientConn(
	rpcAddress string, tlsCertificate []byte,
) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithDefaultCallOptions(maxMsgRecvSize)}
	if withTLS := len(tlsCertificate) > 0; !withTLS {
		opts = append(opts, grpc.WithInsecure())
	} else {
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(tlsCertificate) {
			return nil, fmt.Errorf("credentials: failed to append certificates")
		}
		tlsCreds := credentials.NewTLS(&tls.Config{RootCAs: cp})
		opts = append(opts, grpc.WithTransportCredentials(tlsCreds))
	}

	conn, err := grpc.Dial(rpcAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to RPC server: %v",
			err)
	}

	return conn, nil
}