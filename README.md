## WireGuard (with peering negotiation) over Keybase

### Disclaimer

This repository is not an official Keybase product. This is an experiment. It will likely not work for you.

### Idea

(Keybase service and KBFS has to be running in the background.)

User selects a Keybase team and "connects to team VPN" - right now a using CLI command with team name as an argument. kb-wireguard sets up a [WireGuard](https://www.wireguard.com/) device, and negotiates peering using Keybase chat. There is a `peers.json` file stored in team's KBFS share which maps username+device name to virtual network IPv4 address.

Example `peers.json`:
```
[
    { "username": "zaputest", "device": "Serv 1", "ip": "100.0.0.1" },
    { "username": "zaputest", "device": "Serv 2", "ip": "100.0.0.2" },
    { "username": "zaputest", "device": "Linux Host", "ip": "100.0.0.3" }
]
```

IP addresses are mapped per device (not per user). This way, a single user can use this to connect all of their devices, no matter where physically they are and what public network they are connected to.

### Benefits of using Keybase

There is a central resource that can store peering source of truth that's protected by users' and teams' signature chains. Keybase can't inject new peers into users' VPNs.

Keybase offers a file sharing service that can be used to store configuration, as well as real time text chat service that can be used for presence notifications and pubkey / endpoint announcements.

### Peer list management

Successful peering depends on every client managing a list of peers locally. Initially it's loaded from `peers.json`. Note lack of endpoint addresses and public keys - what's in `peers.json` is not enough to establish a connection.

When a peer comes on-line (`kb-wireguard` tool is launched), the following happen:
1) Load `peers.json`. See if current device can peer with that team, if not, abort. *(TODO: clients should not have to be in the peers table to participate in the network to support "VPN to the servers but not each other" scenario)*
2) Setup a WireGuard device with a public/private key pair (new key pair every time).
3) Fetch recent messages from `#announce` channel of team's chat on Keybase. Match messages to peers loaded from `peers.json`. Add peers to WireGuard config and sync it. At this point we should be able to connect to these peers using VPN IP addresses.
4) Send a message to `#announce` channel with our endpoint IP and public key.

Example "announce" message looks like this:
```
ANNOUNCE 94.130.0.10:7321 jc+Ipv9/W4B6WD/EuVsFMVQjMcBYFfiw5NJD28ffqzE=
```
They are being exchanged using "CHAT" topic type for easier debugging, but the plan is to just move to "DEV".

### Code layout

- `cmd/kb-wireguard` - Main entry point to the program. Does setup and runs background tasks.
- `cmd/run-dev` - Separate program, ran as super user, to setup WireGuard device using `ip` and `wg` commands. Receives configuration updates (peer list) over named pipe and synchronizes it using `wg syncconf` command. Removes WireGuard device after INT or TERM signal.
- `devowner/wireguard.go` - Utilities for `run-dev` to interact with `wg` command.
- `kbwg/program.go` - Types that hold current state of `kb-wireguard` program.
- `kbwg/peerlist.go` - Types for peer list and functions to load them from KBFS and serialize to WireGuard config compatible types to send to `run-dev`.
- `kbwg/announce.go` - Reading and sending announcements through Keybase chat.
- `kbwg/keybase.go` - Keybase utilities that were not available in `go-keybase-chat-bot` library.
- `kbwg/run_dev_owner.go` - Runs and communicates with `run-dev` program. `run-dev` is ran with stdin passed from `kb-wireguard` process so interactive `sudo` works.
- `libpipe` - Types and functions for `kb-wireguard` and `run-dev` to communicate through named pipe.
- `libwireguard` - More helper functions and types to interact with WireGuard config file and `wg` command.

### Problems / TODOs

1) Announcing just one endpoint through Keybase chat is not enough to peer with everyone. It works if you have confirmed, working, public ip address and port. Consider the following scenarios which need more effort:
    - You have a punchable NAT, but it turns out some peers are in the same local network as you are. You won't be able to connect to them using their punched IP/Port.
    - (Similar to above) you have more than one network interface and connection to some peers is better through one interface than the other. There should be a zero-config way of establishing these for the user.
    - Someone is behind NAT that's not connectable to. TURN(-like) connection negotiation is required.
    - To solve this, keep the announcements in `#announce` channel, but do additional negotiation steps that peers will do either out of band (but still on Keybase), or in the team channel.

2) Add a way of connection to VPN without being in `peers.json`. Consider an organization that has some servers and want people to access them via VPN. 
    - There would be a `foo_org.vpn` team for that with all sysadmins and servers in there. 
    - But only servers should be defined in `peers.json` with static IP addresses.
    - Everyone else would need a dynamically assigned IP address, in same subnet.
        - But there is no central authority to assign them. Clients would need to randomize addresses themselves and resolve conflicts E.g. if two announced same random address at roughly the same time, look at messageId, lower wins. Loser recognizes it lost and re-rolls, winner doesn't have to do anything.

### License:

No license yet. All rights reserved. Please don't use this, it's not ready.
