// Package target expands user-supplied arguments (single IPs, hostnames,
// or CIDR blocks) into a stable, deduplicated list of ping targets.
package target

import (
	"fmt"
	"net/netip"
	"strings"
)

// Target is one host that the pinger goroutines will probe. ID is a stable
// display string and map key; Host is the value passed to the resolver.
// For IPs the two are identical; for hostnames both hold the input string
// and DNS resolution is deferred to the pinger.
type Target struct {
	ID   string
	Host string
}

// Expand parses args (in order) into targets. Arguments may be single IPs
// (v4 or v6), CIDR prefixes, or hostnames. Duplicates are removed while
// preserving first-seen order. If the resulting count would exceed
// maxHosts, an error is returned before any pinger is started.
func Expand(args []string, maxHosts int) ([]Target, error) {
	if maxHosts < 1 {
		return nil, fmt.Errorf("max-hosts must be >= 1, got %d", maxHosts)
	}

	var out []Target
	seen := map[string]struct{}{}
	add := func(id, host string) {
		if _, dup := seen[id]; dup {
			return
		}
		seen[id] = struct{}{}
		out = append(out, Target{ID: id, Host: host})
	}

	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}

		switch {
		case strings.Contains(arg, "/"):
			prefix, err := netip.ParsePrefix(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q: %w", arg, err)
			}
			prefix = prefix.Masked()
			count := prefixHostCount(prefix)
			remaining := uint64(maxHosts - len(out))
			if count > remaining {
				return nil, fmt.Errorf("CIDR %s expands to %d hosts; would exceed --max-hosts=%d", arg, count, maxHosts)
			}
			for _, a := range usableHosts(prefix) {
				s := a.String()
				add(s, s)
			}

		default:
			if a, err := netip.ParseAddr(arg); err == nil {
				s := a.String()
				add(s, s)
			} else if looksLikeHostname(arg) {
				add(arg, arg)
			} else {
				return nil, fmt.Errorf("unrecognised target %q (not an IP, CIDR, or hostname)", arg)
			}
			if len(out) > maxHosts {
				return nil, fmt.Errorf("%d targets exceeds --max-hosts=%d", len(out), maxHosts)
			}
		}
	}
	return out, nil
}

// usableHosts walks every address in prefix. For IPv4 prefixes shorter
// than /31 the network and broadcast addresses are omitted, matching
// conventional "host" addresses. /31 (RFC 3021) and /32 keep all bits,
// as does any IPv6 prefix.
func usableHosts(prefix netip.Prefix) []netip.Addr {
	var all []netip.Addr
	a := prefix.Addr()
	for prefix.Contains(a) {
		all = append(all, a)
		next := a.Next()
		if !next.IsValid() {
			break
		}
		a = next
	}
	if prefix.Addr().Is4() && prefix.Bits() < 31 && len(all) >= 2 {
		return all[1 : len(all)-1]
	}
	return all
}

// prefixHostCount returns how many addresses usableHosts would yield,
// without actually walking the prefix. Used as a safety check before
// committing to an expensive walk (a stray /16 or /64).
func prefixHostCount(p netip.Prefix) uint64 {
	addrBits := 32
	if p.Addr().Is6() {
		addrBits = 128
	}
	hostBits := addrBits - p.Bits()
	if hostBits >= 64 {
		return ^uint64(0)
	}
	count := uint64(1) << uint(hostBits)
	if p.Addr().Is4() && p.Bits() < 31 && count >= 2 {
		return count - 2
	}
	return count
}

// looksLikeHostname is a cheap syntactic check; actual DNS resolution
// happens later in the pinger so transient resolver failures don't
// block startup.
func looksLikeHostname(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	if s[0] == '-' || s[0] == '.' || s[len(s)-1] == '-' || s[len(s)-1] == '.' {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-':
		default:
			return false
		}
	}
	return true
}
