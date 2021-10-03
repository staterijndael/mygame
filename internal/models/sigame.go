package models

import "encoding/xml"

type Package struct {
	XMLName    xml.Name `xml:"package"`
	Text       string   `xml:",chardata"`
	Name       string   `xml:"name,attr"`
	Version    string   `xml:"version,attr"`
	ID         string   `xml:"id,attr"`
	Date       string   `xml:"date,attr"`
	Difficulty string   `xml:"difficulty,attr"`
	Xmlns      string   `xml:"xmlns,attr"`
	Info       *Info    `xml:"info"`
	Rounds     *Rounds  `xml:"rounds"`
}

type Info struct {
	Text    string   `xml:",chardata"`
	Authors *Authors `xml:"authors"`
}

type Authors struct {
	Text   string `xml:",chardata"`
	Author string `xml:"author"`
}

type Rounds struct {
	Text  string   `xml:",chardata"`
	Round []*Round `xml:"round"`
}

type Round struct {
	Text   string  `xml:",chardata"`
	Name   string  `xml:"name,attr"`
	Type   string  `xml:"type,attr"`
	Themes *Themes `xml:"themes"`
}

type Themes struct {
	Text  string   `xml:",chardata"`
	Theme []*Theme `xml:"theme"`
}

type Theme struct {
	Text      string     `xml:",chardata"`
	Name      string     `xml:"name,attr"`
	Questions *Questions `xml:"questions"`
}

type Questions struct {
	Text     string      `xml:",chardata"`
	Question []*Question `xml:"question"`
}

type Question struct {
	Text     string    `xml:",chardata"`
	Price    string    `xml:"price,attr"`
	Scenario *Scenario `xml:"scenario"`
	Right    *Right    `xml:"right"`
	Type     *Type     `xml:"type"`
}

type Scenario struct {
	Text string  `xml:",chardata"`
	Atom []*Atom `xml:"atom"`
}

type Atom struct {
	Text string `xml:",chardata"`
	Type string `xml:"type,attr"`
}

type Right struct {
	Text   string   `xml:",chardata"`
	Answer []string `xml:"answer"`
}

type Type struct {
	Text  string   `xml:",chardata"`
	Name  string   `xml:"name,attr"`
	Param []*Param `xml:"param"`
}

type Param struct {
	Text string `xml:",chardata"`
	Name string `xml:"name,attr"`
}
