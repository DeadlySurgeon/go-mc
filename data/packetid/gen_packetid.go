//go:build generate
// +build generate

//gen_packetid.go generates the enumeration of packet IDs used on the wire.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"text/template"

	"github.com/iancoleman/strcase"
)

const (
	version     = "1.17.1"
	protocolURL = "https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/" + version + "/protocol.json"
	//language=gohtml
	packetidTmpl = `// This file is automatically generated by gen_packetIDs.go. DO NOT EDIT.

package packetid

// Login state
const (
	// Clientbound
{{range $ID, $Name := .Login.Clientbound}}	{{$Name}} = {{$ID | printf "%#x"}}
{{end}}
	// Serverbound
{{range $ID, $Name := .Login.Serverbound}}	{{$Name}} = {{$ID | printf "%#x"}}
{{end}}
)

// Ping state
const (
	// Clientbound
{{range $ID, $Name := .Status.Clientbound}}	{{$Name}} = {{$ID | printf "%#x"}}
{{end}}
	// Serverbound
{{range $ID, $Name := .Status.Serverbound}}	{{$Name}} = {{$ID | printf "%#x"}}
{{end}}
)

// Play state
const (
	// Clientbound
{{range $ID, $Name := .Play.Clientbound}}	{{$Name}} = {{$ID | printf "%#x"}}
{{end}}
	// Serverbound
{{range $ID, $Name := .Play.Serverbound}}	{{$Name}} = {{$ID | printf "%#x"}}
{{end}}
)
`
)

// unnest is a utility function to unpack a value from a nested map, given
// an arbitrary set of keys to reach through.
func unnest(input map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	for _, k := range keys {
		sub, ok := input[k]
		if !ok {
			return nil, fmt.Errorf("key %q not found", k)
		}
		next, ok := sub.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("key %q was %T, expected a string map", k, sub)
		}
		input = next
	}
	return input, nil
}

type duplexMappings struct {
	Clientbound map[int32]string
	Serverbound map[int32]string
}

func (m *duplexMappings) EnsureUniqueNames() {
	// Assemble a slice of keys to check across both maps, because we cannot
	// mutate a map while iterating it.
	clientBounds := make(map[string]int32)
	for sk, sv := range m.Clientbound {
		clientBounds[sv] = sk
	}
	for sk, sv := range m.Serverbound {
		if ck, ok := clientBounds[sv]; ok {
			m.Clientbound[ck] = sv + "Clientbound"
			m.Serverbound[sk] = sv + "Serverbound"
		}
	}
}

// unpackMapping returns the set of packet IDs and their names for a given
// game state.
func unpackMapping(data map[string]interface{}, gameState string) (duplexMappings, error) {
	out := duplexMappings{
		Clientbound: make(map[int32]string),
		Serverbound: make(map[int32]string),
	}

	info, err := unnest(data, gameState, "toClient", "types")
	if err != nil {
		return duplexMappings{}, err
	}
	pType := info["packet"].([]interface{})[1].([]interface{})[0].(map[string]interface{})["type"]
	mappings := pType.([]interface{})[1].(map[string]interface{})["mappings"].(map[string]interface{})
	for k, v := range mappings {
		out.Clientbound[mustAtoi(k)] = strcase.ToCamel(v.(string))
	}
	info, err = unnest(data, gameState, "toServer", "types")
	if err != nil {
		return duplexMappings{}, err
	}
	pType = info["packet"].([]interface{})[1].([]interface{})[0].(map[string]interface{})["type"]
	mappings = pType.([]interface{})[1].(map[string]interface{})["mappings"].(map[string]interface{})
	for k, v := range mappings {
		out.Serverbound[mustAtoi(k)] = strcase.ToCamel(v.(string))
	}

	return out, nil
}

func mustAtoi(num string) int32 {
	if n, err := strconv.ParseInt(num, 0, 32); err != nil {
		panic(err)
	} else {
		return int32(n)
	}
}

type protocolIDs struct {
	Login  duplexMappings
	Play   duplexMappings
	Status duplexMappings
	// Handshake state has no packets
}

func downloadInfo() (*protocolIDs, error) {
	resp, err := http.Get(protocolURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var out protocolIDs
	if out.Login, err = unpackMapping(data, "login"); err != nil {
		return nil, fmt.Errorf("login: %v", err)
	}
	out.Login.EnsureUniqueNames()
	if out.Play, err = unpackMapping(data, "play"); err != nil {
		return nil, fmt.Errorf("play: %v", err)
	}
	out.Play.EnsureUniqueNames()
	if out.Status, err = unpackMapping(data, "status"); err != nil {
		return nil, fmt.Errorf("play: %v", err)
	}
	out.Status.EnsureUniqueNames()

	return &out, nil
}

//go:generate go run $GOFILE
//go:generate go fmt packetid.go
func main() {
	fmt.Println("generating packetid.go")
	pIDs, err := downloadInfo()
	if err != nil {
		panic(err)
	}

	f, err := os.Create("packetid.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("packetIDs").Parse(packetidTmpl))
	if err := tmpl.Execute(f, pIDs); err != nil {
		panic(err)
	}
}
