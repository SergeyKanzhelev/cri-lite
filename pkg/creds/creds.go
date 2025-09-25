// Package creds provides a custom gRPC credentials implementation that extracts the caller's PID.
package creds

import (
	"context"
	"net"

	"google.golang.org/grpc/credentials"
)

// PIDCreds is a custom gRPC credentials implementation that extracts the caller's PID.
type PIDCreds struct{}

// NewPIDCreds creates a new PIDCreds.
func NewPIDCreds() credentials.TransportCredentials {
	return &PIDCreds{}
}

// ClientHandshake implements the credentials.TransportCredentials interface.
func (c *PIDCreds) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, nil, nil
}

// ServerHandshake implements the credentials.TransportCredentials interface.
func (c *PIDCreds) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	ucred, err := getUcred(conn)
	if err != nil {
		return conn, nil, err
	}

	return conn, &ucredAuthInfo{ucred: ucred}, nil
}

// Info implements the credentials.TransportCredentials interface.
func (c *PIDCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "pid",
		SecurityVersion:  "1.0",
	}
}

// Clone makes a copy of PIDCreds.
func (c *PIDCreds) Clone() credentials.TransportCredentials {
	return &PIDCreds{}
}

// OverrideServerName overrides the server name.
func (c *PIDCreds) OverrideServerName(serverNameOverride string) error {
	return nil
}

type ucredAuthInfo struct {
	ucred *ucred
}

func (ai *ucredAuthInfo) AuthType() string {
	return "ucred"
}

func (ai *ucredAuthInfo) GetPID() int32 {
	return ai.ucred.pid
}
