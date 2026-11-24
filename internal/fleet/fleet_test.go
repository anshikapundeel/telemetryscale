package fleet

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var bg = context.Background()

func tctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(bg, 3*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestTLSExpiryAndChain(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "https://")
	if r := (TLSCheck{MinDays: 1, Insecure: true}).Run(tctx(t), addr); !r.OK {
		t.Fatalf("insecure+valid expiry should pass: %s", r.Detail)
	}
	if r := (TLSCheck{MinDays: 1}).Run(tctx(t), addr); r.OK {
		t.Fatal("self-signed chain must fail verification")
	}
	if r := (TLSCheck{MinDays: 365 * 200, Insecure: true}).Run(tctx(t), addr); r.OK {
		t.Fatal("expiry below MinDays must fail")
	}
}

func fakeServer(t *testing.T, handle func(net.Conn)) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); handle(c) }()
		}
	}()
	return l.Addr().String()
}

func TestSMTPCheck(t *testing.T) {
	addr := fakeServer(t, func(c net.Conn) {
		c.Write([]byte("220 mail ready\r\n"))
		buf := make([]byte, 64)
		c.Read(buf)
		c.Write([]byte("250-mail\r\n250 SIZE 100\r\n"))
	})
	if r := (SMTPCheck{}).Run(tctx(t), addr); !r.OK {
		t.Fatalf("smtp should pass: %s", r.Detail)
	}
	bad := fakeServer(t, func(c net.Conn) { c.Write([]byte("554 no\r\n")) })
	if r := (SMTPCheck{}).Run(tctx(t), bad); r.OK {
		t.Fatal("bad banner must fail")
	}
}

func TestHAProxyCSV(t *testing.T) {
	hdr := "# pxname,svname,qcur,status\n"
	good := hdr + "web,FRONTEND,0,OPEN\nweb,a,0,UP\nweb,b,0,UP\nweb,BACKEND,0,UP\n"
	if r := parseHAProxyCSV(strings.NewReader(good)); !r.OK {
		t.Fatalf("all up should pass: %s", r.Detail)
	}
	bad := hdr + "web,a,0,UP\nweb,b,0,DOWN\n"
	r := parseHAProxyCSV(strings.NewReader(bad))
	if r.OK || !strings.Contains(r.Detail, "web/b") {
		t.Fatalf("down server must fail and be named: %+v", r)
	}
}

func TestPuppetFreshnessAndFailures(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	c := PuppetCheck{MaxAge: time.Hour}
	fresh := "---\ntime:\n  last_run: 1999999000\nevents:\n  failure: 0\n"
	if r := c.evaluate(fresh, now); !r.OK {
		t.Fatalf("fresh run should pass: %s", r.Detail)
	}
	stale := "time:\n  last_run: 1999990000\nevents:\n  failure: 0\n"
	if r := c.evaluate(stale, now); r.OK {
		t.Fatal("stale run must fail")
	}
	future := "time:\n  last_run: 2000100000\nevents:\n  failure: 0\n"
	if r := c.evaluate(future, now); r.OK {
		t.Fatal("last_run far in the future must fail (clock skew)")
	}
	failed := "time:\n  last_run: 1999999900\nevents:\n  failure: 3\n"
	if r := c.evaluate(failed, now); r.OK {
		t.Fatal("failed resources must fail")
	}
}

func TestDHCPUtilisation(t *testing.T) {
	now := time.Unix(1000, 0)
	leases := "2000 aa:bb 10.0.0.2 h1 *\n2000 cc:dd 10.0.0.3 h2 *\n10 ee:ff 10.0.0.4 old *\n"
	if r := (DHCPCheck{PoolSize: 10, MaxUtil: 0.9}).evaluate(strings.NewReader(leases), now); !r.OK {
		t.Fatalf("20%% used should pass: %s", r.Detail)
	}
	if r := (DHCPCheck{PoolSize: 2, MaxUtil: 0.9}).evaluate(strings.NewReader(leases), now); r.OK {
		t.Fatal("100%% used must fail")
	}
}

func TestKerberosTicket(t *testing.T) {
	now := time.Date(2026, 11, 24, 10, 0, 0, 0, time.Local)
	c := KerberosCheck{MinTicketLife: time.Hour}
	out := "Valid starting     Expires            Service principal\n11/24/26 09:00:00  11/24/26 19:00:00  krbtgt/EXAMPLE.COM@EXAMPLE.COM\n"
	if r := c.evaluateTicket(out, now); !r.OK {
		t.Fatalf("9h left should pass: %s", r.Detail)
	}
	soon := strings.ReplaceAll(out, "19:00:00", "10:30:00")
	if r := c.evaluateTicket(soon, now); r.OK {
		t.Fatal("30m left must fail")
	}
	expired := strings.ReplaceAll(out, "19:00:00", "09:30:00")
	if r := c.evaluateTicket(expired, now); r.OK {
		t.Fatal("expired must fail")
	}
}

func TestLDAPBind(t *testing.T) {
	okAddr := fakeServer(t, func(c net.Conn) {
		buf := make([]byte, 128)
		c.Read(buf)
		c.Write([]byte{0x30, 0x0c, 0x02, 0x01, 0x01, 0x61, 0x07, 0x0a, 0x01, 0x00, 0x04, 0x00, 0x04, 0x00})
	})
	if r := (LDAPCheck{}).Run(tctx(t), okAddr); !r.OK {
		t.Fatalf("success bind should pass: %s", r.Detail)
	}
	badAddr := fakeServer(t, func(c net.Conn) {
		buf := make([]byte, 128)
		c.Read(buf)
		c.Write([]byte{0x30, 0x0c, 0x02, 0x01, 0x01, 0x61, 0x07, 0x0a, 0x01, 0x31, 0x04, 0x00, 0x04, 0x00})
	})
	r := (LDAPCheck{BindDN: "cn=x", Password: "bad"}).Run(tctx(t), badAddr)
	if r.OK || !strings.Contains(r.Detail, "49") {
		t.Fatalf("invalid credentials (49) must fail: %+v", r)
	}
}

func TestDNSCheckFailureAndSuccess(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			q := buf[:n]
			// question section only: labels up to the zero byte, then qtype+qclass
			i := 12
			for i < len(q) && q[i] != 0 {
				i += int(q[i]) + 1
			}
			qend := i + 1 + 4
			qtype := uint16(q[qend-4])<<8 | uint16(q[qend-3])
			an := byte(0)
			if qtype == 1 { // A record only; AAAA gets an empty NOERROR answer
				an = 1
			}
			resp := append([]byte{}, q[:2]...)
			resp = append(resp, 0x81, 0x80, 0, 1, 0, an, 0, 0, 0, 0)
			resp = append(resp, q[12:qend]...)
			if an == 1 {
				resp = append(resp, 0xC0, 0x0C, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 127, 0, 0, 1)
			}
			pc.WriteTo(resp, addr)
		}
	}()
	good := DNSCheck{Server: pc.LocalAddr().String(), MaxLatency: time.Second}
	if r := good.Run(tctx(t), "example.test"); !r.OK {
		t.Fatalf("lookup via fake resolver should pass: %s", r.Detail)
	}
	dead := DNSCheck{Server: "127.0.0.1:1"}
	c, cancel := context.WithTimeout(bg, 500*time.Millisecond)
	defer cancel()
	if r := dead.Run(c, "example.test"); r.OK {
		t.Fatal("unreachable resolver must fail")
	}
}

type stub struct{ ok bool }

func (stub) Name() string { return "stub" }
func (s stub) Run(context.Context, string) Result {
	time.Sleep(20 * time.Millisecond)
	return Result{OK: s.ok}
}

func TestRunAllIsConcurrentAndOrdered(t *testing.T) {
	var jobs []Job
	for i := 0; i < 20; i++ {
		jobs = append(jobs, Job{Check: stub{ok: i%2 == 0}, Target: string(rune('a' + i))})
	}
	start := time.Now()
	rs := RunAll(bg, jobs, 10, time.Second)
	if el := time.Since(start); el > 200*time.Millisecond {
		t.Fatalf("not concurrent: %v", el)
	}
	for i, r := range rs {
		if r.Target != string(rune('a'+i)) || r.OK != (i%2 == 0) {
			t.Fatalf("result %d out of order or wrong: %+v", i, r)
		}
	}
}
