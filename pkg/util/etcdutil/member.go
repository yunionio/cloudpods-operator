package etcdutil

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type Member struct {
	Name string
	// Kubernetes namespace this member runs in.
	Namespace string
	// ID field can be 0, which is unknown ID.
	// We know the ID of a member when we get the member information from etcd,
	// but not from Kubernetes pod list.
	ID uint64

	SecurePeer   bool
	SecureClient bool
	// IPv6Cluster indicates whether this member should listen on IPv6 addresses
	IPv6Cluster bool
}

func (m *Member) Addr() string {
	return fmt.Sprintf("%s.%s.%s.svc", m.Name, clusterNameFromMemberName(m.Name), m.Namespace)
}

// ClientURL is the client URL for this member
func (m *Member) ClientURL() string {
	return fmt.Sprintf("%s://%s:2379", m.clientScheme(), m.Addr())
}

func (m *Member) clientScheme() string {
	if m.SecureClient {
		return "https"
	}
	return "http"
}

func (m *Member) peerScheme() string {
	if m.SecurePeer {
		return "https"
	}
	return "http"
}

func (m *Member) ListenClientURL() string {
	if m.IPv6Cluster {
		return fmt.Sprintf("%s://[::]:2379", m.clientScheme())
	}
	return fmt.Sprintf("%s://0.0.0.0:2379", m.clientScheme())
}
func (m *Member) ListenPeerURL() string {
	if m.IPv6Cluster {
		return fmt.Sprintf("%s://[::]:2380", m.peerScheme())
	}
	return fmt.Sprintf("%s://0.0.0.0:2380", m.peerScheme())
}

func (m *Member) PeerURL() string {
	return fmt.Sprintf("%s://%s:2380", m.peerScheme(), m.Addr())
}

type MemberSet map[string]*Member

func NewMemberSet(ms ...*Member) MemberSet {
	res := MemberSet{}
	for _, m := range ms {
		res[m.Name] = m
	}
	return res
}

// the set of all members of s1 that are not members of s2
func (ms MemberSet) Diff(other MemberSet) MemberSet {
	diff := MemberSet{}
	for n, m := range ms {
		if _, ok := other[n]; !ok {
			diff[n] = m
		}
	}
	return diff
}

// IsEqual tells whether two member sets are equal by checking
// - they have the same set of members and member equality are judged by Name only.
func (ms MemberSet) IsEqual(other MemberSet) bool {
	if ms.Size() != other.Size() {
		return false
	}
	for n := range ms {
		if _, ok := other[n]; !ok {
			return false
		}
	}
	return true
}

func (ms MemberSet) Size() int {
	return len(ms)
}

func (ms MemberSet) String() string {
	var mstring []string

	for m := range ms {
		mstring = append(mstring, m)
	}
	return strings.Join(mstring, ",")
}

func (ms MemberSet) PickOne() *Member {
	for _, m := range ms {
		return m
	}
	panic("empty")
}

func (ms MemberSet) PeerURLPairs() []string {
	ps := make([]string, 0)
	for _, m := range ms {
		ps = append(ps, fmt.Sprintf("%s=%s", m.Name, m.PeerURL()))
	}
	return ps
}

func (ms MemberSet) Add(m *Member) {
	ms[m.Name] = m
}

func (ms MemberSet) Remove(name string) {
	delete(ms, name)
}

func (ms MemberSet) ClientURLs() []string {
	endpoints := make([]string, 0, len(ms))
	for _, m := range ms {
		endpoints = append(endpoints, m.ClientURL())
	}
	return endpoints
}

var validPeerURL = regexp.MustCompile(`^\w+:\/\/[\w\.\-]+(:\d+)?$`)

func MemberNameFromPeerURL(pu string) (string, error) {
	// url.Parse has very loose validation. We do our own validation.
	if !validPeerURL.MatchString(pu) {
		return "", errors.New("invalid PeerURL format")
	}
	u, err := url.Parse(pu)
	if err != nil {
		return "", err
	}
	path := strings.Split(u.Host, ":")[0]
	name := strings.Split(path, ".")[0]
	return name, err
}

func clusterNameFromMemberName(mn string) string {
	i := strings.LastIndex(mn, "-")
	if i == -1 {
		panic(fmt.Sprintf("unexpected member name: %s", mn))
	}
	return mn[:i]
}
