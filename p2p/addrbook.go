package p2p

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"code.aliyun.com/chain33/chain33/common/crypto"
	pb "code.aliyun.com/chain33/chain33/types"
)

//peer address manager
type AddrBook struct {
	mtx      sync.Mutex
	ourAddrs map[string]*NetAddress
	addrPeer map[string]*knownAddress
	filePath string
	key      string
	Quit     chan struct{}
}

type knownAddress struct {
	kmtx        sync.Mutex
	Addr        *NetAddress
	Attempts    uint
	LastAttempt time.Time
	LastSuccess time.Time
}

func (a *AddrBook) getPeerStat(addr string) *knownAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	if peer, ok := a.addrPeer[addr]; ok {
		return peer
	}
	return nil

}

func NewAddrBook(filePath string) *AddrBook {
	peers := make(map[string]*knownAddress, 0)
	a := &AddrBook{
		ourAddrs: make(map[string]*NetAddress),
		addrPeer: peers,
		filePath: filePath,
		Quit:     make(chan struct{}),
	}

	a.init()
	a.Start()
	return a
}

func (a *AddrBook) setKey(key string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.key = key
}

func (a *AddrBook) getKey() string {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.key
}

func (a *AddrBook) init() {
	c, err := crypto.New(pb.GetSignatureTypeName(pb.SECP256K1))
	if err != nil {
		log.Error("CryPto Error", "Error", err.Error())
		return
	}

	key, err := c.GenKey()
	if err != nil {
		log.Error("GenKey", "Error", err)
		return
	}
	a.setKey(hex.EncodeToString((key.Bytes())))
}
func newKnownAddress(addr *NetAddress) *knownAddress {
	return &knownAddress{
		Addr:        addr,
		Attempts:    0,
		LastAttempt: time.Now(),
	}
}
func (ka *knownAddress) markGood() {
	ka.kmtx.Lock()
	defer ka.kmtx.Unlock()
	now := time.Now()
	ka.LastAttempt = now
	ka.Attempts = 0
	ka.LastSuccess = now
}

func (ka *knownAddress) Copy() *knownAddress {
	ka.kmtx.Lock()
	defer ka.kmtx.Unlock()
	copytmp := *ka
	copytmp.Addr = copytmp.Addr.Copy()
	return &copytmp
}

func (ka *knownAddress) markAttempt() {
	ka.kmtx.Lock()
	defer ka.kmtx.Unlock()
	now := time.Now()
	ka.LastAttempt = now
	ka.Attempts += 1
}
func (ka *knownAddress) flushPeerStatus(m *Monitor) {
	ka.kmtx.Lock()
	defer ka.kmtx.Unlock()
	ka.Attempts = m.GetCount()
	ka.LastAttempt = time.Unix(int64(m.GetLastOp()), 0)
	ka.LastSuccess = time.Unix(int64(m.GetLastOk()), 0)
}

func (ka *knownAddress) GetAttempts() uint {
	ka.kmtx.Lock()
	defer ka.kmtx.Unlock()
	return ka.Attempts
}

// OnStart implements Service.
func (a *AddrBook) Start() error {
	a.loadFromFile()
	go a.saveRoutine()
	return nil
}

func (a *AddrBook) AddOurAddress(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	log.Info("Add our address to book", "addr", addr)
	a.ourAddrs[addr.String()] = addr
}
func (a *AddrBook) Size() int {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return len(a.addrPeer)
}

type addrBookJSON struct {
	Key   string
	Addrs []*knownAddress
}

func (a *AddrBook) saveToFile(filePath string) {
	log.Info("Saving AddrBook to file", "size", a.Size())

	a.mtx.Lock()
	defer a.mtx.Unlock()
	// Compile Addrs
	addrs := []*knownAddress{}
	for _, ka := range a.addrPeer {
		addrs = append(addrs, ka.Copy())
	}
	if len(addrs) == 0 {
		return
	}
	aJSON := &addrBookJSON{
		Key:   a.key,
		Addrs: addrs,
	}

	jsonBytes, err := json.MarshalIndent(aJSON, "", "\t")
	if err != nil {
		log.Error("Failed to save AddrBook to file", "err", err)
		return
	}
	log.Info("saveToFile", string(jsonBytes), "")

	err = a.writeFile(filePath, jsonBytes, 0755)
	if err != nil {
		log.Error("Error", "Failed to save AddrBook to file", "file", filePath, "err", err)
	}

}

func (a *AddrBook) writeFile(filePath string, bytes []byte, mode os.FileMode) error {
	dir := filepath.Dir(filePath)
	f, err := ioutil.TempFile(dir, "")
	if err != nil {
		return err
	}
	_, err = f.Write(bytes)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if permErr := os.Chmod(f.Name(), mode); err == nil {
		err = permErr
	}
	if err == nil {
		err = os.Rename(f.Name(), filePath)
	}
	// any err should result in full cleanup
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}

// Returns false if file does not exist.
// cmn.Panics if file is corrupt.
func (a *AddrBook) loadFromFile() bool {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	_, err := os.Stat(a.filePath)
	if os.IsNotExist(err) {
		return false
	}

	r, err := os.Open(a.filePath)
	if err != nil {
		log.Crit("Error opening file %s: %v", a.filePath, err)
	}
	defer r.Close()
	aJSON := &addrBookJSON{}
	dec := json.NewDecoder(r)
	err = dec.Decode(aJSON)
	if err != nil {
		log.Crit("Error reading file %s: %v", a.filePath, err)
	}

	a.key = aJSON.Key

	for _, ka := range aJSON.Addrs {
		a.addrPeer[ka.Addr.String()] = ka
	}

	return true
}

// Save saves the book.
func (a *AddrBook) Save() {
	log.Info("Saving AddrBook to file", "size", a.Size())

	a.saveToFile(a.filePath)
}

func (a *AddrBook) saveRoutine() {
	dumpAddressTicker := time.NewTicker(10 * time.Second)
out:
	for {
		select {
		case <-dumpAddressTicker.C:
			a.saveToFile(a.filePath)
		case <-a.Quit:
			break out
		}
	}
	dumpAddressTicker.Stop()
	a.saveToFile(a.filePath)
	log.Warn("Address handler done")
}

func (a *AddrBook) Stop() {
	a.Quit <- struct{}{}
}
func (a *AddrBook) addAddress(addr *NetAddress) {
	if addr == nil {
		return
	}

	if !addr.Routable() {
		log.Error("Cannot add non-routable address %v", addr)
		return
	}
	if _, ok := a.ourAddrs[addr.String()]; ok {
		// Ignore our own listener address.
		return
	}
	//已经添加的不重复添加
	if _, ok := a.addrPeer[addr.String()]; ok {
		return
	}

	ka := newKnownAddress(addr)
	a.addrPeer[ka.Addr.String()] = ka
	return
}

// NOTE: addr must not be nil
func (a *AddrBook) AddAddress(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	log.Info("Add address to book", "addr", addr)
	a.addAddress(addr)
}

func (a *AddrBook) RemoveAddr(peeraddr string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	log.Warn("RemoveAddr", "peer", peeraddr)
	if _, ok := a.addrPeer[peeraddr]; ok {
		delete(a.addrPeer, peeraddr)
	}
}

func (a *AddrBook) GetPeers() []*NetAddress {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	peerlist := make([]*NetAddress, 0)
	for _, peer := range a.addrPeer {
		peerlist = append(peerlist, peer.Addr)
	}
	return peerlist
}

func (a *AddrBook) GetAddrs() []string {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	addrlist := make([]string, 0)
	for _, peer := range a.addrPeer {
		if peer.GetAttempts() == 0 {
			addrlist = append(addrlist, peer.Addr.String())
		}

	}
	return addrlist
}
