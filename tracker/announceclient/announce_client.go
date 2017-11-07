package announceclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"code.uber.internal/infra/kraken/lib/peercontext"
	"code.uber.internal/infra/kraken/lib/serverset"
	"code.uber.internal/infra/kraken/torlib"
	"code.uber.internal/infra/kraken/utils/httputil"
	"github.com/jackpal/bencode-go"
)

// Client defines a client for announcing and getting peers.
type Client interface {
	Announce(name string, h torlib.InfoHash, downloaded int64) ([]torlib.PeerInfo, error)
}

type client struct {
	config  Config
	pctx    peercontext.PeerContext
	servers serverset.Set
}

// New creates a new Client.
func New(config Config, pctx peercontext.PeerContext, servers serverset.Set) Client {
	return &client{config.applyDefaults(), pctx, servers}
}

// Default creates a default Client.
func Default(pctx peercontext.PeerContext, servers serverset.Set) Client {
	return New(Config{}, pctx, servers)
}

// Announce announces the torrent identified by (name, h) with the number of
// downloaded bytes. Returns a list of all other peers announcing for said torrent,
// sorted by priority.
func (c *client) Announce(name string, h torlib.InfoHash, downloaded int64) ([]torlib.PeerInfo, error) {
	v := url.Values{}

	v.Add("name", name)
	v.Add("info_hash", h.String())
	v.Add("peer_id", c.pctx.PeerID.String())
	v.Add("port", strconv.Itoa(c.pctx.Port))
	v.Add("ip", c.pctx.IP)
	v.Add("dc", c.pctx.Zone)
	v.Add("downloaded", strconv.FormatInt(downloaded, 10))

	// TODO(codyg): Remove these from tracker.
	v.Add("uploaded", "0")
	v.Add("left", "0")

	q := v.Encode()

	var err error
	for it := c.servers.Iter(); it.HasNext(); it.Next() {
		var resp *http.Response
		resp, err = httputil.Get(
			fmt.Sprintf("http://%s/announce?%s", it.Addr(), q),
			httputil.SendTimeout(c.config.Timeout))
		if err != nil {
			continue
		}
		var b struct {
			Peers []torlib.PeerInfo `bencode:"peers"`
		}
		if err := bencode.Unmarshal(resp.Body, &b); err != nil {
			return nil, fmt.Errorf("unmarshal failed: %s", err)
		}
		return b.Peers, nil
	}
	return nil, err
}