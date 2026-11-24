package fleet

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------- TLS: handshake, chain validity, expiry ----------

type TLSCheck struct {
	MinDays  int  // fail if the leaf expires sooner than this
	Insecure bool // skip chain verification (lab self-signed certs); expiry still checked
}

func (TLSCheck) Name() string { return "tls" }

func (c TLSCheck) Run(ctx context.Context, target string) Result {
	d := tls.Dialer{Config: &tls.Config{InsecureSkipVerify: c.Insecure}}
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return fail("handshake failed: " + err.Error())
	}
	defer conn.Close()
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return fail("no peer certificate")
	}
	days := int(time.Until(certs[0].NotAfter).Hours() / 24)
	if days < c.MinDays {
		return fail(fmt.Sprintf("certificate expires in %d days (min %d)", days, c.MinDays))
	}
	return ok(fmt.Sprintf("expires in %d days", days))
}

// ---------- DNS: resolution success and latency against a specific server ----------

type DNSCheck struct {
	Server     string // host:port of the resolver under test
	MaxLatency time.Duration
}

func (DNSCheck) Name() string { return "dns" }

func (c DNSCheck) Run(ctx context.Context, name string) Result {
	r := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "udp", c.Server)
	}}
	start := time.Now()
	addrs, err := r.LookupHost(ctx, name)
	lat := time.Since(start)
	if err != nil {
		return fail("lookup failed: " + err.Error())
	}
	if c.MaxLatency > 0 && lat > c.MaxLatency {
		return fail(fmt.Sprintf("slow resolution %v > %v", lat.Round(time.Millisecond), c.MaxLatency))
	}
	return ok(fmt.Sprintf("%s -> %s in %v", name, strings.Join(addrs, ","), lat.Round(time.Millisecond)))
}

// ---------- SMTP: banner + EHLO against a relay ----------

type SMTPCheck struct{}

func (SMTPCheck) Name() string { return "smtp" }

func (SMTPCheck) Run(ctx context.Context, target string) Result {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return fail("connect failed: " + err.Error())
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	br := bufio.NewReader(conn)
	banner, err := br.ReadString('\n')
	if err != nil || !strings.HasPrefix(banner, "220") {
		return fail("bad banner: " + strings.TrimSpace(banner))
	}
	fmt.Fprint(conn, "EHLO fleetguard\r\n")
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return fail("EHLO read failed: " + err.Error())
		}
		if !strings.HasPrefix(line, "250") {
			return fail("EHLO rejected: " + strings.TrimSpace(line))
		}
		if len(line) >= 4 && line[3] == ' ' { // final line of multi-line reply
			break
		}
	}
	fmt.Fprint(conn, "QUIT\r\n")
	return ok("relay accepted EHLO")
}

// ---------- HAProxy: backend health from the stats CSV ----------

type HAProxyCheck struct{}

func (HAProxyCheck) Name() string { return "haproxy" }

func (HAProxyCheck) Run(ctx context.Context, statsURL string) Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statsURL, nil)
	if err != nil {
		return fail(err.Error())
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("stats unreachable: " + err.Error())
	}
	defer resp.Body.Close()
	return parseHAProxyCSV(resp.Body)
}

func parseHAProxyCSV(r io.Reader) Result {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil || len(rows) < 2 {
		return fail("cannot parse stats CSV")
	}
	hdr := rows[0]
	col := map[string]int{}
	for i, h := range hdr {
		col[strings.TrimPrefix(h, "# ")] = i
	}
	px, sv, st := col["pxname"], col["svname"], col["status"]
	var up, down int
	var downs []string
	for _, row := range rows[1:] {
		if len(row) <= st || row[sv] == "FRONTEND" || row[sv] == "BACKEND" {
			continue
		}
		if strings.HasPrefix(row[st], "UP") {
			up++
		} else {
			down++
			downs = append(downs, row[px]+"/"+row[sv])
		}
	}
	if down > 0 {
		return fail(fmt.Sprintf("%d backend servers down: %s", down, strings.Join(downs, ", ")))
	}
	return ok(fmt.Sprintf("%d backend servers up", up))
}

// ---------- Puppet: last-run freshness and failures ----------

type PuppetCheck struct{ MaxAge time.Duration }

func (PuppetCheck) Name() string { return "puppet" }

func (c PuppetCheck) Run(_ context.Context, path string) Result {
	b, err := os.ReadFile(path)
	if err != nil {
		return fail("cannot read last_run_summary: " + err.Error())
	}
	return c.evaluate(string(b), time.Now())
}

var kvLine = regexp.MustCompile(`^\s*([a-z_]+):\s*"?([0-9.]+)"?\s*$`)

func (c PuppetCheck) evaluate(summary string, now time.Time) Result {
	vals := map[string]float64{}
	for _, line := range strings.Split(summary, "\n") {
		if m := kvLine.FindStringSubmatch(line); m != nil {
			f, _ := strconv.ParseFloat(m[2], 64)
			vals[m[1]] = f
		}
	}
	lr, have := vals["last_run"]
	if !have {
		return fail("no last_run in summary")
	}
	age := now.Sub(time.Unix(int64(lr), 0))
	if age < -5*time.Minute {
		return fail(fmt.Sprintf("last_run is %v in the future (clock skew or bad summary)", (-age).Round(time.Minute)))
	}
	if c.MaxAge > 0 && age > c.MaxAge {
		return fail(fmt.Sprintf("last puppet run %v ago (max %v)", age.Round(time.Minute), c.MaxAge))
	}
	if f := vals["failure"]; f > 0 {
		return fail(fmt.Sprintf("%d failed resources in last run", int(f)))
	}
	return ok(fmt.Sprintf("converged %v ago, no failures", age.Round(time.Minute)))
}

// ---------- DHCP: lease pool utilisation from a dnsmasq-style lease file ----------

type DHCPCheck struct {
	PoolSize int     // total addresses in the pool
	MaxUtil  float64 // fail above this fraction, e.g. 0.9
}

func (DHCPCheck) Name() string { return "dhcp" }

func (c DHCPCheck) Run(_ context.Context, leaseFile string) Result {
	f, err := os.Open(leaseFile)
	if err != nil {
		return fail("cannot read lease file: " + err.Error())
	}
	defer f.Close()
	return c.evaluate(f, time.Now())
}

func (c DHCPCheck) evaluate(r io.Reader, now time.Time) Result {
	active := 0
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 3 {
			continue
		}
		exp, err := strconv.ParseInt(fs[0], 10, 64)
		if err != nil {
			continue
		}
		if exp == 0 || time.Unix(exp, 0).After(now) { // 0 = infinite lease in dnsmasq
			active++
		}
	}
	if c.PoolSize <= 0 {
		return fail("pool size not configured")
	}
	util := float64(active) / float64(c.PoolSize)
	if util > c.MaxUtil {
		return fail(fmt.Sprintf("pool %.0f%% used (%d/%d), limit %.0f%%", util*100, active, c.PoolSize, c.MaxUtil*100))
	}
	return ok(fmt.Sprintf("pool %.0f%% used (%d/%d)", util*100, active, c.PoolSize))
}

// ---------- Kerberos: KDC reachability and ticket expiry ----------

type KerberosCheck struct {
	MinTicketLife time.Duration
	// Klist is injectable for tests; defaults to running `klist`.
	Klist func(ctx context.Context) (string, error)
}

func (KerberosCheck) Name() string { return "kerberos" }

// Target "kdc:88" checks KDC TCP reachability; target "ticket" checks the local TGT.
func (c KerberosCheck) Run(ctx context.Context, target string) Result {
	if target != "ticket" {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", target)
		if err != nil {
			return fail("KDC unreachable: " + err.Error())
		}
		conn.Close()
		return ok("KDC reachable")
	}
	klist := c.Klist
	if klist == nil {
		klist = func(ctx context.Context) (string, error) {
			b, err := exec.CommandContext(ctx, "klist").CombinedOutput()
			return string(b), err
		}
	}
	out, err := klist(ctx)
	if err != nil {
		return fail("no valid ticket cache: " + strings.TrimSpace(out))
	}
	return c.evaluateTicket(out, time.Now())
}

var tgtLine = regexp.MustCompile(`(\d\d/\d\d/\d{2,4} \d\d:\d\d:\d\d)\s+(\d\d/\d\d/\d{2,4} \d\d:\d\d:\d\d)\s+krbtgt/`)

func (c KerberosCheck) evaluateTicket(out string, now time.Time) Result {
	m := tgtLine.FindStringSubmatch(out)
	if m == nil {
		return fail("no krbtgt ticket found")
	}
	layout := "01/02/06 15:04:05"
	if len(strings.Fields(m[2])[0]) == 10 {
		layout = "01/02/2006 15:04:05"
	}
	exp, err := time.ParseInLocation(layout, m[2], time.Local)
	if err != nil {
		return fail("cannot parse expiry: " + err.Error())
	}
	left := exp.Sub(now)
	if left <= 0 {
		return fail("TGT expired " + (-left).Round(time.Minute).String() + " ago")
	}
	if left < c.MinTicketLife {
		return fail(fmt.Sprintf("TGT expires in %v (min %v)", left.Round(time.Minute), c.MinTicketLife))
	}
	return ok(fmt.Sprintf("TGT valid for %v", left.Round(time.Minute)))
}

// ---------- LDAP: minimal BER simple-bind (anonymous or DN+password) ----------

type LDAPCheck struct {
	BindDN, Password string
}

func (LDAPCheck) Name() string { return "ldap" }

func berLen(n int) []byte {
	if n < 128 {
		return []byte{byte(n)}
	}
	return []byte{0x81, byte(n)} // supports up to 255, enough for a bind request
}

func tlv(tag byte, body []byte) []byte {
	return append(append([]byte{tag}, berLen(len(body))...), body...)
}

func bindRequest(dn, pw string) []byte {
	body := append([]byte{0x02, 0x01, 0x03}, tlv(0x04, []byte(dn))...)
	body = append(body, tlv(0x80, []byte(pw))...)
	bind := tlv(0x60, body)
	return tlv(0x30, append([]byte{0x02, 0x01, 0x01}, bind...))
}

func (c LDAPCheck) Run(ctx context.Context, target string) Result {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return fail("connect failed: " + err.Error())
	}
	defer conn.Close()
	if dl, okd := ctx.Deadline(); okd {
		_ = conn.SetDeadline(dl)
	}
	if _, err := conn.Write(bindRequest(c.BindDN, c.Password)); err != nil {
		return fail("write failed: " + err.Error())
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return fail("no bind response: " + err.Error())
	}
	return parseBindResponse(buf[:n])
}

// parseBindResponse finds the BindResponse (0x61) and reads its resultCode.
func parseBindResponse(b []byte) Result {
	for i := 0; i+4 < len(b); i++ {
		if b[i] == 0x61 {
			// 0x61 len 0x0a 0x01 code
			j := i + 2
			if b[i+1]&0x80 != 0 {
				j = i + 2 + int(b[i+1]&0x7f)
			}
			if j+2 < len(b) && b[j] == 0x0a && b[j+1] == 0x01 {
				if code := b[j+2]; code == 0 {
					return ok("bind succeeded")
				} else {
					return fail(fmt.Sprintf("bind failed, LDAP result code %d", code))
				}
			}
		}
	}
	return fail("unrecognised LDAP response")
}
