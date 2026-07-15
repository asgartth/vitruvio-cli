package main

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// mxGraphModel — estrutura achatada do draw.io (mxGraph).
type mxGraphModel struct {
	Cells []mxCell `xml:"root>mxCell"`
}
type mxCell struct {
	ID     string `xml:"id,attr"`
	Value  string `xml:"value,attr"`
	Vertex string `xml:"vertex,attr"`
	Edge   string `xml:"edge,attr"`
	Source string `xml:"source,attr"`
	Target string `xml:"target,attr"`
}

var reDiagram = regexp.MustCompile(`(?s)<diagram[^>]*>(.*?)</diagram>`)
var reTags = regexp.MustCompile(`<[^>]+>`)

// inflaDrawio decodifica o conteúdo comprimido (base64 + raw deflate + urlencode).
func inflaDrawio(b64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return "", err
	}
	r := flate.NewReader(bytes.NewReader(data))
	out, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		return "", err
	}
	if dec, err := url.QueryUnescape(string(out)); err == nil {
		return dec, nil
	}
	return string(out), nil
}

func limpaRotulo(s string) string {
	s = reTags.ReplaceAllString(s, " ") // draw.io permite HTML no value
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// parseDrawio decodifica um .drawio/.xml e resume nós e arestas em texto limpo.
func parseDrawio(conteudo string) (string, error) {
	ms := reDiagram.FindAllStringSubmatch(conteudo, -1)
	if len(ms) == 0 {
		// pode ser um mxGraphModel puro, sem <mxfile>
		ms = [][]string{{"", conteudo}}
	}
	var b strings.Builder
	for pg, m := range ms {
		inner := strings.TrimSpace(m[1])
		xmlStr := inner
		if !strings.Contains(inner, "<mxGraphModel") {
			if dec, err := inflaDrawio(inner); err == nil && strings.Contains(dec, "<mxGraphModel") {
				xmlStr = dec
			}
		}
		var g mxGraphModel
		if err := xml.Unmarshal([]byte(xmlStr), &g); err != nil {
			continue
		}
		rotulo := map[string]string{}
		var nos, arestas []mxCell
		for _, c := range g.Cells {
			if c.Vertex == "1" {
				rotulo[c.ID] = limpaRotulo(c.Value)
				nos = append(nos, c)
			} else if c.Edge == "1" {
				arestas = append(arestas, c)
			}
		}
		if len(ms) > 1 {
			b.WriteString(fmt.Sprintf("### Página %d\n", pg+1))
		}
		b.WriteString(fmt.Sprintf("Nós (%d):\n", len(nos)))
		for _, n := range nos {
			r := rotulo[n.ID]
			if r == "" {
				r = "(sem rótulo)"
			}
			b.WriteString("- " + r + "\n")
		}
		b.WriteString(fmt.Sprintf("Conexões (%d):\n", len(arestas)))
		for _, e := range arestas {
			s, t := rotulo[e.Source], rotulo[e.Target]
			if s == "" {
				s = e.Source
			}
			if t == "" {
				t = e.Target
			}
			lbl := limpaRotulo(e.Value)
			if lbl != "" {
				lbl = " [" + lbl + "]"
			}
			b.WriteString(fmt.Sprintf("- %s → %s%s\n", s, t, lbl))
		}
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("não encontrei mxGraphModel válido no arquivo")
	}
	return b.String(), nil
}
