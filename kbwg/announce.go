package kbwg

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type AnnounceMsg struct {
	// Device making the announcement.
	Device KBDev
	// Peer announces their endpoint, should be ip:port.
	Endpoint string
}

const AnnounceChatName = "announce"

func ParseAnnounceMsg() {

}

func AnnounceFindChat(mctx MetaContext) (ret chat1.ConvSummary, err error) {
	list, err := mctx.API().GetConversations(false)
	if err != nil {
		return ret, fmt.Errorf("AnnounceFindChat failed to call GetConversations: %w", err)
	}

	for _, conv := range list {
		ch := conv.Channel
		if ch.MembersType == "team" && ch.Name == mctx.Prog.KeybaseTeam && ch.TopicType == "chat" && ch.TopicName == AnnounceChatName {
			return conv, nil
		}
	}
	return ret, fmt.Errorf("Failed to find chat @%s#%s", mctx.Prog.KeybaseTeam, AnnounceChatName)
}

func FindAnnouncements(mctx MetaContext) error {
	messages, err := mctx.API().GetTextMessages(mctx.Prog.AnnounceChannel, false)
	if err != nil {
		return err
	}
	announcementsFound := make(map[KBDev]struct{}) // only take first announcement for each user
	for _, msg := range messages {
		kbdev := KBDev{
			Device:   msg.Sender.DeviceName,
			Username: msg.Sender.Username,
		}
		peer, ok := mctx.Prog.KeybasePeers[kbdev]
		if !ok {
			// Sender is not a peer
			continue
		}
		if _, alreadyFound := announcementsFound[kbdev]; alreadyFound {
			// Already had an announcement from this sender.
			continue
		}
		announcementsFound[kbdev] = struct{}{}
		peer.LastAnnouncement = msg.Content.Text.Body
		mctx.Prog.KeybasePeers[kbdev] = peer

		fmt.Printf("+ %v is announcing %s\n", kbdev, msg.Content.Text.Body)
	}
	spew.Dump(mctx.Prog.KeybasePeers)
	return nil
}

func SendAnnouncement(mctx MetaContext) error {
	text := fmt.Sprintf("ANNOUNCE %s", mctx.Prog.Endpoint)
	_, err := mctx.API().SendMessage(mctx.Prog.AnnounceChannel, text)
	if err != nil {
		return fmt.Errorf("SendAnnouncement couldn't SendMessage: %w", err)
	}
	return nil
}
