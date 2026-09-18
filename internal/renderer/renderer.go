package renderer

import (
	"fmt"
	"html/template"
	"math"
	"net/url"
	"regexp"
	"strings"
)

type Renderer struct {
	templates *template.Template
}

type CounterData struct {
	Value       int64
	Label       string
	Theme       string
	Color       string
	HomepageURL string
}

type BadgeData struct {
	Label       string
	Value       string
	Color       string
	Style       string
	HomepageURL string
}

// shieldLayout is the computed geometry handed to the SVG template. Text
// coordinates (LabelX, ValueX, *Len) live in the scale(0.1) space the
// template renders text in, i.e. 10x the pixel values.
type shieldLayout struct {
	Label       string
	Value       string
	Color       string
	HomepageURL string
	Gradient    bool
	Width       float64
	LabelWidth  float64
	ValueWidth  float64
	LabelX      float64
	ValueX      float64
	LabelLen    float64
	ValueLen    float64
}

func New() *Renderer {
	return &Renderer{
		templates: loadTemplates(),
	}
}

func loadTemplates() *template.Template {
	tmpl := template.New("svg")
	tmpl = template.Must(tmpl.New("shield").Parse(shieldTemplate))
	return tmpl
}

func (r *Renderer) RenderCounter(data CounterData) (string, error) {
	data.Color = normalizeColor(data.Color)
	if data.Color == "" {
		data.Color = "#007bff"
	}

	return r.renderShield(shieldLayout{
		Label:       data.Label,
		Value:       fmt.Sprintf("%d", data.Value),
		Color:       data.Color,
		Gradient:    data.Theme != "flat",
		HomepageURL: prepareHomepage(data.HomepageURL),
	})
}

func (r *Renderer) RenderBadge(data BadgeData) (string, error) {
	data.Color = normalizeColor(data.Color)
	if data.Color == "" {
		data.Color = "#007bff"
	}

	return r.renderShield(shieldLayout{
		Label:       data.Label,
		Value:       data.Value,
		Color:       data.Color,
		Gradient:    data.Style != "flat",
		HomepageURL: prepareHomepage(data.HomepageURL),
	})
}

const textPadding = 10 // horizontal padding around each text section, px

func (r *Renderer) renderShield(layout shieldLayout) (string, error) {
	if layout.Label != "" {
		layout.LabelWidth = round1(textWidth(layout.Label) + textPadding)
	}
	layout.ValueWidth = round1(textWidth(layout.Value) + textPadding)
	layout.Width = round1(layout.LabelWidth + layout.ValueWidth)
	layout.LabelX = round1(layout.LabelWidth / 2 * 10)
	layout.ValueX = round1((layout.LabelWidth + layout.ValueWidth/2) * 10)
	layout.LabelLen = round1((layout.LabelWidth - textPadding) * 10)
	layout.ValueLen = round1((layout.ValueWidth - textPadding) * 10)

	var buf strings.Builder
	if err := r.templates.ExecuteTemplate(&buf, "shield", layout); err != nil {
		return "", fmt.Errorf("failed to render shield: %w", err)
	}
	return buf.String(), nil
}

// textWidth approximates the rendered width of s in 11px Verdana/DejaVu Sans,
// the font stack the shield template uses. textLength/lengthAdjust in the
// template absorbs the remaining estimation error.
func textWidth(s string) float64 {
	var w float64
	for _, r := range s {
		w += charWidth(r)
	}
	return w
}

func charWidth(r rune) float64 {
	switch {
	case r >= '0' && r <= '9':
		return 7.0
	case strings.ContainsRune("ijl.,:;!|'", r):
		return 3.7
	case strings.ContainsRune("frtI()[]- ", r):
		return 4.6
	case strings.ContainsRune("mwMW@", r):
		return 10.5
	case r >= 'A' && r <= 'Z':
		return 8.0
	case r > 127: // CJK and other wide glyphs
		return 11.0
	default:
		return 6.6
	}
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}

// shields.io-compatible color names
var namedColors = map[string]string{
	"brightgreen": "#4c1",
	"green":       "#97ca00",
	"yellowgreen": "#a4a61d",
	"yellow":      "#dfb317",
	"orange":      "#fe7d37",
	"red":         "#e05d44",
	"blue":        "#007ec6",
	"lightgrey":   "#9f9f9f",
	"lightgray":   "#9f9f9f",
	"grey":        "#555",
	"gray":        "#555",
	"purple":      "#7c3aed",
}

var hexColorPattern = regexp.MustCompile(`^(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// normalizeColor turns query-param color values into valid SVG fills: shields
// color names and bare hex like "7c3aed" ("#" cannot appear in a URL query).
// Anything else passes through for CSS to interpret.
func normalizeColor(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if hex, ok := namedColors[strings.ToLower(value)]; ok {
		return hex
	}
	if hexColorPattern.MatchString(value) {
		return "#" + value
	}
	return ""
}

func prepareHomepage(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		if !strings.Contains(value, "://") {
			parsed, err = url.Parse("https://" + value)
		}
	}
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}

	return (&url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   "/",
	}).String()
}

const shieldTemplate = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.Width}}" height="20">{{if .Gradient}}<linearGradient id="smooth" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>{{end}}<clipPath id="round"><rect fill="#fff" height="20" rx="3" width="{{.Width}}"/></clipPath><g clip-path="url(#round)">{{if .Label}}<rect fill="#595959" height="20" width="{{.LabelWidth}}"/>{{end}}<rect fill="{{.Color}}" height="20" width="{{.ValueWidth}}" x="{{.LabelWidth}}"/>{{if .Gradient}}<rect fill="url(#smooth)" height="20" width="{{.Width}}"/>{{end}}</g><g fill="#fff" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110" text-anchor="middle">{{if .Label}}<text fill="#010101" fill-opacity=".3" lengthAdjust="spacing" textLength="{{.LabelLen}}" transform="scale(0.1)" x="{{.LabelX}}" y="150">{{.Label}}</text><text lengthAdjust="spacing" textLength="{{.LabelLen}}" transform="scale(0.1)" x="{{.LabelX}}" y="140">{{.Label}}</text>{{end}}<text fill="#010101" fill-opacity=".3" lengthAdjust="spacing" textLength="{{.ValueLen}}" transform="scale(0.1)" x="{{.ValueX}}" y="150">{{.Value}}</text><text lengthAdjust="spacing" textLength="{{.ValueLen}}" transform="scale(0.1)" x="{{.ValueX}}" y="140">{{.Value}}</text>{{if .HomepageURL}}{{if .Label}}<a href="{{.HomepageURL}}" xlink:href="{{.HomepageURL}}"><rect fill="rgba(0,0,0,0)" height="20" width="{{.LabelWidth}}"/></a>{{end}}<a href="{{.HomepageURL}}" xlink:href="{{.HomepageURL}}"><rect fill="rgba(0,0,0,0)" height="20" width="{{.ValueWidth}}" x="{{.LabelWidth}}"/></a>{{end}}</g></svg>`
