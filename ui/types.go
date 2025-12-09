package ui

import "github.com/frostbyte57/GoPipe/internal/wormhole"

type ErrorMsg error
type TransferDoneMsg struct {
	Filename string
}
type HandshakeSuccessMsg []byte
type BackToMenuMsg struct{}

type ConnectedMsg struct {
	Code   string
	Client *wormhole.Client
}

type TxProgressMsg struct {
	Current int64
	Total   int64
	Ratio   float64
}
