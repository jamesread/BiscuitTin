package rss

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxSummaryRunes = 400

var (
	htmlTag         = regexp.MustCompile(`(?is)<[^>]*>`)
	htmlBlock       = regexp.MustCompile(`(?is)</?(p|div|br|li|h[1-6]|tr)[^>]*>`)
	htmlComment     = regexp.MustCompile(`(?is)<!--.*?-->`)
	xmlEncoding     = regexp.MustCompile(`(?i)encoding\s*=\s*["']([^"']+)["']`)
	xmlEncodingDecl = regexp.MustCompile(`(?i)encoding\s*=\s*["'][^"']+["']`)
	httpCharset     = regexp.MustCompile(`(?i)charset=([^;]+)`)
)

type Item struct {
	Feed      string    `json:"feed"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Link      string    `json:"link"`
	Published time.Time `json:"published"`
}

type rssDoc struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Content     string `xml:"encoded"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Date        string `xml:"date"`
}

type atomDoc struct {
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Links     []atomLink `xml:"link"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

func parseFeed(raw []byte, fallbackName string) (string, []Item, error) {
	return parseFeedTyped(raw, "", fallbackName)
}

func parseFeedTyped(raw []byte, contentType, fallbackName string) (string, []Item, error) {
	raw = decodeToUTF8(raw, contentType)
	raw = xmlEncodingDecl.ReplaceAll(raw, []byte(`encoding="UTF-8"`))
	if looksLikeAtom(raw) {
		name, items, err := parseAtom(raw, fallbackName)
		if err == nil && len(items) > 0 {
			return name, items, nil
		}
	}
	name, items, err := parseRSS(raw, fallbackName)
	if err == nil && len(items) > 0 {
		return name, items, nil
	}
	if name, items, aerr := parseAtom(raw, fallbackName); aerr == nil && len(items) > 0 {
		return name, items, nil
	}
	if err != nil {
		return "", nil, err
	}
	return name, items, nil
}

func looksLikeAtom(raw []byte) bool {
	head := string(raw)
	if len(head) > 800 {
		head = head[:800]
	}
	return strings.Contains(head, "<feed") && strings.Contains(head, "<entry")
}

func parseRSS(raw []byte, fallbackName string) (string, []Item, error) {
	doc := &rssDoc{}
	if err := xml.Unmarshal(raw, doc); err != nil {
		return "", nil, err
	}
	name := firstNonEmpty(plainText(doc.Channel.Title), fallbackName, "RSS")
	items := make([]Item, 0, len(doc.Channel.Items))
	for _, it := range doc.Channel.Items {
		item, ok := newItem(name, it.Title, it.Description, it.Content, it.Link, firstNonEmpty(it.PubDate, it.Date))
		if ok {
			items = append(items, item)
		}
	}
	return name, items, nil
}

func parseAtom(raw []byte, fallbackName string) (string, []Item, error) {
	doc := &atomDoc{}
	if err := xml.Unmarshal(raw, doc); err != nil {
		return "", nil, err
	}
	name := firstNonEmpty(plainText(doc.Title), fallbackName, "RSS")
	items := make([]Item, 0, len(doc.Entries))
	for _, it := range doc.Entries {
		item, ok := newItem(name, it.Title, it.Summary, it.Content, atomHref(it.Links), firstNonEmpty(it.Published, it.Updated))
		if ok {
			items = append(items, item)
		}
	}
	return name, items, nil
}

func newItem(feed, title, summary, content, link, published string) (Item, bool) {
	title = plainText(title)
	if title == "" {
		return Item{}, false
	}
	body := plainText(summary)
	if body == "" {
		body = plainText(content)
	}
	return Item{
		Feed:      feed,
		Title:     title,
		Summary:   clipRunes(body, maxSummaryRunes),
		Link:      strings.TrimSpace(link),
		Published: parsePublished(published),
	}, true
}

func atomHref(links []atomLink) string {
	alt := ""
	for _, l := range links {
		if l.Href == "" {
			continue
		}
		if l.Rel == "" || l.Rel == "alternate" {
			return l.Href
		}
		if alt == "" {
			alt = l.Href
		}
	}
	return alt
}

func plainText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = unescapeEntities(s)
	s = htmlComment.ReplaceAllString(s, " ")
	s = htmlBlock.ReplaceAllString(s, " ")
	s = htmlTag.ReplaceAllString(s, " ")
	s = unescapeEntities(s)
	s = mapLegacyPunct(s)
	return strings.Join(strings.Fields(s), " ")
}

func unescapeEntities(s string) string {
	for i := 0; i < 4; i++ {
		next := html.UnescapeString(s)
		if next == s {
			break
		}
		s = next
	}
	return s
}

// cp1252 is the C1 controls 0x80–0x9F as Windows-1252. RSS feeds often emit
// &#146; (and friends) meaning a curly quote, which html.UnescapeString leaves
// as a control character.
var cp1252 = [32]rune{
	'€', 0, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0, 'Ž', 0,
	0, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0, 'ž', 'Ÿ',
}

func mapLegacyPunct(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x80 && r <= 0x9f {
			if m := cp1252[r-0x80]; m != 0 {
				r = m
			} else {
				continue
			}
		}
		if r == unicode.ReplacementChar {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func decodeToUTF8(raw []byte, contentType string) []byte {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	name := strings.ToLower(strings.TrimSpace(charsetOf(raw, contentType)))
	name = strings.Trim(name, `"'`)
	switch name {
	case "", "utf-8", "utf8":
		if utf8.Valid(raw) {
			return raw
		}
		return decodeWindows1252(raw)
	case "iso-8859-1", "latin1", "latin-1", "iso8859-1", "windows-1252", "cp1252":
		return decodeWindows1252(raw)
	default:
		if utf8.Valid(raw) {
			return raw
		}
		return decodeWindows1252(raw)
	}
}

func charsetOf(raw []byte, contentType string) string {
	if m := httpCharset.FindStringSubmatch(contentType); len(m) == 2 {
		return m[1]
	}
	head := raw
	if len(head) > 200 {
		head = head[:200]
	}
	if m := xmlEncoding.FindSubmatch(head); len(m) == 2 {
		return string(m[1])
	}
	return ""
}

func decodeWindows1252(raw []byte) []byte {
	var b strings.Builder
	b.Grow(len(raw) + len(raw)/4)
	for _, c := range raw {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c <= 0x9f:
			if r := cp1252[c-0x80]; r != 0 {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(rune(c))
		}
	}
	return []byte(b.String())
}

func clipRunes(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	r = r[:n]
	return strings.TrimSpace(string(r)) + "…"
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

var publishedLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	time.RFC822Z,
	time.RFC822,
	"Mon, 02 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parsePublished(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range publishedLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// AgeLabel is a short relative time for a living-room display.
func AgeLabel(published, now time.Time) string {
	if published.IsZero() {
		return ""
	}
	if published.After(now) {
		published = now
	}
	d := now.Sub(published)
	switch {
	case d < time.Minute:
		return "Just now"
	case d < time.Hour:
		n := int(d / time.Minute)
		return countLabel(n, "minute") + " ago"
	case d < 24*time.Hour:
		n := int(d / time.Hour)
		return countLabel(n, "hour") + " ago"
	case d < 48*time.Hour:
		return "Yesterday"
	case d < 7*24*time.Hour:
		n := int(d / (24 * time.Hour))
		return countLabel(n, "day") + " ago"
	case published.Year() == now.Year():
		return published.Format("2 Jan")
	default:
		return published.Format("2 Jan 2006")
	}
}

func countLabel(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
