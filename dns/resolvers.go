package dns

import "sync"

type LookUp struct {
	Hostname string
	IP       string
}

type Resolver struct {
	DnsServer string
	buff      []*LookUp
	mutex     sync.Mutex
}

func (r *Resolver) Add(lookup *LookUp) {
	r.mutex.Lock()
	if len(r.buff) < 20 {
		r.buff = append(r.buff, lookup)
	} else {
		r.Resolve()
	}
	r.mutex.Unlock()
}

func (r *Resolver) Resolve() {

}
