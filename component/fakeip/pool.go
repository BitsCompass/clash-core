package fakeip

import (
	"errors"
	"net"
	"sync"

	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/component/trie"
)

// Pool is a implementation about fake ip generator without storage
type Pool struct {
	max     uint32
	min     uint32
	gateway uint32
	offset  uint32
	mux     sync.Mutex
	host    *trie.DomainTrie
	ipnet   *net.IPNet
	cache   *cache.LruCache
}

// Lookup return a fake ip with host
func (p *Pool) Lookup(host string) net.IP {
	p.mux.Lock()
	defer p.mux.Unlock()
	if elm, exist := p.cache.Get(host); exist {
		ip := elm.(net.IP)

		// ensure ip --> host on head of linked list
		n := ipToUint(ip.To4())
		offset := n - p.min + 1
		p.cache.Get(offset)
		return ip
	}

	ip := p.get(host)
	p.cache.Set(host, ip)
	return ip
}

// LookBack return host with the fake ip
func (p *Pool) LookBack(ip net.IP) (string, bool) {
	p.mux.Lock()
	defer p.mux.Unlock()

	if ip = ip.To4(); ip == nil {
		return "", false
	}

	n := ipToUint(ip.To4())
	offset := n - p.min + 1

	if elm, exist := p.cache.Get(offset); exist {
		host := elm.(string)

		// ensure host --> ip on head of linked list
		p.cache.Get(host)
		return host, true
	}

	return "", false
}

// LookupHost return if domain in host
func (p *Pool) LookupHost(domain string) bool {
	if p.host == nil {
		return false
	}
	return p.host.Search(domain) != nil
}

// Exist returns if given ip exists in fake-ip pool
func (p *Pool) Exist(ip net.IP) bool {
	p.mux.Lock()
	defer p.mux.Unlock()

	if ip = ip.To4(); ip == nil {
		return false
	}

	n := ipToUint(ip.To4())
	offset := n - p.min + 1
	return p.cache.Exist(offset)
}

// Gateway return gateway ip
func (p *Pool) Gateway() net.IP {
	return uintToIP(p.gateway)
}

// IPNet return raw ipnet
func (p *Pool) IPNet() *net.IPNet {
	return p.ipnet
}

// PatchFrom clone cache from old pool
func (p *Pool) PatchFrom(o *Pool) {
	o.cache.CloneTo(p.cache)
}

func (p *Pool) get(host string) net.IP {
	current := p.offset
	for {
		p.offset = (p.offset + 1) % (p.max - p.min)
		// Avoid infinite loops
		if p.offset == current {
			break
		}

		if !p.cache.Exist(p.offset) {
			break
		}
	}
	ip := uintToIP(p.min + p.offset - 1)
	p.cache.Set(p.offset, host)
	return ip
}

func ipToUint(ip net.IP) uint32 {
	v := uint32(ip[0]) << 24
	v += uint32(ip[1]) << 16
	v += uint32(ip[2]) << 8
	v += uint32(ip[3])
	return v
}

func uintToIP(v uint32) net.IP {
	return net.IP{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// New return Pool instance
func New(ipnet *net.IPNet, size int, host *trie.DomainTrie) (*Pool, error) {
	min := ipToUint(ipnet.IP) + 2

	ones, bits := ipnet.Mask.Size()
	total := 1<<uint(bits-ones) - 2

	if total <= 0 {
		return nil, errors.New("ipnet don't have valid ip")
	}

	max := min + uint32(total) - 1
	return &Pool{
		min:     min,
		max:     max,
		gateway: min - 1,
		host:    host,
		ipnet:   ipnet,
		cache:   cache.NewLRUCache(cache.WithSize(size * 2)),
	}, nil
}
