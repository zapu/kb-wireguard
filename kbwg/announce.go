package kbwg

import (
	"fmt"
	"regexp"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/zapu/kb-wireguard/libpipe"
)

type AnnounceMsg struct {
	// Peer announces their endpoint, should be ip:port.
	Endpoint string
	// Public key
	PublicKey string
	SentAt    time.Time
	MessageID chat1.MessageID
}

const AnnounceChatName = "announce"

// ANNOUNCE ip_addr pub_key
var announceChatMsgRxp = regexp.MustCompile(`ANNOUNCE ([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}:[0-9]{1,5}) (.+)`)

func ParseAnnounceMsg(msg string) (ret AnnounceMsg, ok bool) {
	matches := announceChatMsgRxp.FindStringSubmatch(msg)
	fmt.Printf("+ Parsing %s\n", msg)
	if len(matches) > 0 {
		ret.Endpoint = matches[1]
		ret.PublicKey = matches[2]
		return ret, true
	}
	return ret, false
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

// FindAnnouncements queries chat for peer announcement. Call with
// unreadOnly=false initially to get all recent (not older than 1 hour)
// announcements. Then periodically call with unreadOnly=true to get new
// announcements as they are being posted.
func FindAnnouncements(mctx MetaContext, unreadOnly bool) (newAnncs bool, err error) {
	messages, err := mctx.API().GetTextMessages(mctx.Prog.AnnounceChannel, unreadOnly)
	if err != nil {
		return false, err
	}
	cutoff := time.Now().Add(-1 * time.Hour)
	announcementsFound := make(map[KBDev]struct{}) // only take first announcement for each user
	for _, msg := range messages {
		sentAt := time.Unix(msg.SentAt, 0)
		if sentAt.Before(cutoff) {
			// break
		}
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

		if peer.Active && msg.Id <= peer.LastAnnouncement.MessageID {
			// We've already seen this one.
			continue
		}

		announcementsFound[kbdev] = struct{}{}
		parsed, ok := ParseAnnounceMsg(msg.Content.Text.Body)
		if !ok {
			continue
		}
		parsed.SentAt = sentAt
		parsed.MessageID = msg.Id

		peer.Active = true
		peer.Endpoint = parsed.Endpoint
		peer.PublicKey = parsed.PublicKey

		peer.LastAnnouncement = parsed
		mctx.Prog.KeybasePeers[kbdev] = peer

		fmt.Printf("+ %v is announcing %q (msg ID: %d)\n", kbdev, msg.Content.Text.Body, msg.Id)
		newAnncs = true
	}
	return newAnncs, nil
}

func SendAnnouncement(mctx MetaContext) error {
	text := fmt.Sprintf("ANNOUNCE %s %s", mctx.Prog.Endpoint, mctx.Prog.SelfPeer.PublicKey)
	_, err := mctx.API().SendMessage(mctx.Prog.AnnounceChannel, text)
	if err != nil {
		return fmt.Errorf("SendAnnouncement couldn't SendMessage: %w", err)
	}
	return nil
}

func AnnouncementsBgTask(mctx MetaContext) error {
	_, err := FindAnnouncements(mctx, false /* unreadOnly */)
	if err != nil {
		return err
	}

	wgPeers := SerializeWireGuardPeerList(mctx)
	fmt.Printf("Doing initial sync for peer list with %d peer(s).\n", len(wgPeers))
	peersMsg, _ := libpipe.SerializeMsgInterface("peers", wgPeers)
	mctx.Prog.DevRunner.WriteLine(peersMsg)

loop:
	for {
		select {
		case <-time.After(5 * time.Second):
		case <-mctx.Ctx.Done():
			break loop
		}

		new, err := FindAnnouncements(mctx, true /* unreadOnly */)
		if err != nil {
			return err
		}

		if new {
			wgPeers := SerializeWireGuardPeerList(mctx)
			fmt.Printf("Got new announcements, syncing peer list with %d peer(s).\n", len(wgPeers))
			peersMsg, _ := libpipe.SerializeMsgInterface("peers", wgPeers)
			mctx.Prog.DevRunner.WriteLine(peersMsg)
		}
	}

	fmt.Printf("[X] AnnouncementsBgTask stopping\n")
	return nil
}

func SelfAnnouncementBgTask(mctx MetaContext) error {
	err := SendAnnouncement(mctx)
	if err != nil {
		return fmt.Errorf("failed to SendAnnouncement: %w", err)
	}

	for {
		select {
		case <-time.After(30 * time.Minute):
			err := SendAnnouncement(mctx)
			if err != nil {
				return fmt.Errorf("failed to SendAnnouncement: %w", err)
			}
		case <-mctx.Ctx.Done():
			return mctx.Ctx.Err()
		}
	}
}
