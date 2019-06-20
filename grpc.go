package main

import (
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
	"io/ioutil"
	"os/user"
	"path"
)

func grpcClient(a *alias) lnrpc.LightningClient {
	usr, err := user.Current()
	if err != nil {
		logger.logerr("current user fail", err.Error())
		return nil
	}
	tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")
	macaroonPath := a.MacaroonPath

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		logger.logerr("cert failure", err.Error())
		return nil
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
	if err != nil {
		logger.logerr("macaroon file read failure", err.Error())
		return nil
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		return nil
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	}

	host := fmt.Sprintf("localhost:%d", a.Port)
	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		logger.logerr("problem with grpc connection", err.Error())
		return nil
	}
	client := lnrpc.NewLightningClient(conn)
	return client
}

func unlocker(a *alias) lnrpc.WalletUnlockerClient {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Cannot get current user:", err)
		return nil
	}
	tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
		return nil
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
	}

	host := fmt.Sprintf("localhost:%d", a.Port)
	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		fmt.Println("cannot dial to lnd", err)
		return nil
	}
	client := lnrpc.NewWalletUnlockerClient(conn)

	return client
}
