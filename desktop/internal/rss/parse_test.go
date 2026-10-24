package rss

import (
	"strings"
	"testing"
	"time"
)

func TestParseRSS(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Example News</title>
    <item>
      <title>A walk in the Dales</title>
      <description>&lt;p&gt;Hills and a &lt;b&gt;stone&lt;/b&gt; wall.&lt;/p&gt;</description>
      <link>https://example.com/dales</link>
      <pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate>
    </item>
    <item>
      <title></title>
      <description>Missing title should be skipped</description>
    </item>
  </channel>
</rss>`)
	name, items, err := parseFeed(raw, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Example News" {
		t.Fatalf("name = %q", name)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d; want 1", len(items))
	}
	if items[0].Title != "A walk in the Dales" {
		t.Fatalf("title = %q", items[0].Title)
	}
	if items[0].Summary != "Hills and a stone wall." {
		t.Fatalf("summary = %q", items[0].Summary)
	}
	if items[0].Link != "https://example.com/dales" {
		t.Fatalf("link = %q", items[0].Link)
	}
	if items[0].Published.IsZero() {
		t.Fatal("published should parse")
	}
}

func TestParseAtom(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <entry>
    <title>City lights</title>
    <summary>Tokyo at dusk</summary>
    <updated>2006-01-02T15:04:05Z</updated>
    <link rel="alternate" href="https://example.com/tokyo"/>
  </entry>
</feed>`)
	name, items, err := parseFeed(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Atom Feed" || len(items) != 1 {
		t.Fatalf("name=%q items=%d", name, len(items))
	}
	if items[0].Title != "City lights" || items[0].Summary != "Tokyo at dusk" {
		t.Fatalf("item = %+v", items[0])
	}
	if items[0].Link != "https://example.com/tokyo" {
		t.Fatalf("link = %q", items[0].Link)
	}
}

func TestPlainTextStripsHTMLAndEntities(t *testing.T) {
	got := plainText(`<p>Hello&nbsp;&amp;&nbsp;<a href="x">world</a></p>`)
	if got != "Hello & world" {
		t.Fatalf("plainText = %q", got)
	}
	got = plainText(`It&rsquo;s &ldquo;news&rdquo; &mdash; and more&hellip;`)
	if got != "It’s “news” — and more…" {
		t.Fatalf("named entities = %q", got)
	}
	got = plainText(`She said &#8217;hello&#8217;`)
	if got != "She said ’hello’" {
		t.Fatalf("numeric entities = %q", got)
	}
	got = plainText("Windows quote &#146;here&#146;")
	if got != "Windows quote ’here’" {
		t.Fatalf("cp1252 numeric = %q", got)
	}
	got = plainText(`&amp;rsquo;`)
	if got != "’" {
		t.Fatalf("double-encoded = %q", got)
	}
}

func TestDecodeWindows1252Feed(t *testing.T) {
	// "It's news" with a CP1252 curly apostrophe (0x92) and ISO-8859-1 declared.
	raw := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><rss><channel><title>X</title>" +
		"<item><title>It" + string([]byte{0x92}) + "s news</title><description>ok</description></item>" +
		"</channel></rss>")
	_, items, err := parseFeed(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "It’s news" {
		t.Fatalf("items = %+v", items)
	}
}

func TestClipRunes(t *testing.T) {
	got := clipRunes(strings.Repeat("a", 10), 6)
	if got != "aaaaaa…" {
		t.Fatalf("clip = %q", got)
	}
}

func TestAgeLabel(t *testing.T) {
	now := time.Date(2026, 10, 24, 21, 0, 0, 0, time.UTC)
	cases := []struct {
		pub  time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "Just now"},
		{now.Add(-12 * time.Minute), "12 minutes ago"},
		{now.Add(-time.Hour), "1 hour ago"},
		{now.Add(-3 * time.Hour), "3 hours ago"},
		{now.Add(-30 * time.Hour), "Yesterday"},
		{now.Add(-3 * 24 * time.Hour), "3 days ago"},
		{time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "2 Jan"},
		{time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), "15 Mar 2024"},
		{time.Time{}, ""},
	}
	for _, c := range cases {
		if got := AgeLabel(c.pub, now); got != c.want {
			t.Errorf("AgeLabel(%v) = %q; want %q", c.pub, got, c.want)
		}
	}
}
