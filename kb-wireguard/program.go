package main

import (
	"context"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type Program struct {
	API *kbchat.API

	Self         KBDev
	SelfDeviceID string

	KeybaseTeam string

	Endpoint string

	// `KeybasePeers` is a list of peers from peers.txt excluding ourselves. So
	// the actual list of all peers in the VPN is `KeybasePeers` + `SelfPeer`.

	SelfPeer     KeybasePeer
	KeybasePeers map[KBDev]KeybasePeer

	AnnounceChannel chat1.ChatChannel
}

type MetaContext struct {
	Prog *Program
	Ctx  context.Context
}

func (mctx MetaContext) API() *kbchat.API {
	return mctx.Prog.API
}

func (p *Program) MCtxTODO() MetaContext {
	return MetaContext{
		Prog: p,
		Ctx:  context.TODO(),
	}
}

func (p *Program) LoadSelf(ctx context.Context) error {
	kbStatus, err := KeybaseGetLoggedInStatus(p.API)
	if err != nil {
		return err
	}

	p.Self = KBDev{
		Username: kbStatus.Username,
		Device:   kbStatus.Device.Name,
	}
	p.SelfDeviceID = kbStatus.Device.DeviceID
	return nil
}
