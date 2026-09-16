package compiler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/d2lang/d2/d2graph"
	"github.com/d2lang/d2/d2layouts/d2dagrelayout"
	"github.com/d2lang/d2/d2plugin"
	"github.com/d2lang/d2/d2themes/d2themescatalog"
)

// enginePlugins are the layout plugins bundled into the D2 library. They run
// in-process. d2plugin.ListPlugins is deliberately not used because it also
// executes external d2plugin-* binaries found on $PATH.
func enginePlugins() map[string]d2plugin.Plugin {
	initDagre()
	return map[string]d2plugin.Plugin{
		"dagre": &d2plugin.DagrePlugin,
		"elk":   &d2plugin.ELKPlugin,
		"tala":  &d2plugin.TALAPlugin,
	}
}

var engineOrder = []Engine{
	{ID: "dagre", Name: "Dagre", Description: "Fast hierarchical layout (D2 default)", Available: true},
	{ID: "elk", Name: "ELK", Description: "Eclipse Layout Kernel; better for dense graphs", Available: true},
	{ID: "tala", Name: "TALA", Description: "Orthogonal layout for architecture diagrams (MPL-2.0, bundled with D2 0.9+)", Available: true},
}

// Engines lists the layout engines supported by this build.
func Engines() []Engine {
	out := make([]Engine, len(engineOrder))
	copy(out, engineOrder)
	return out
}

// Themes lists D2's themes, taken from the D2 catalog (light first, then dark).
func Themes() []Theme {
	var out []Theme
	for _, t := range d2themescatalog.LightCatalog {
		out = append(out, Theme{ID: t.ID, Name: t.Name})
	}
	for _, t := range d2themescatalog.DarkCatalog {
		out = append(out, Theme{ID: t.ID, Name: t.Name, Dark: true})
	}
	return out
}

func validTheme(id int64) bool {
	for _, t := range Themes() {
		if t.ID == id {
			return true
		}
	}
	return false
}

// resolvers build D2 layout and router resolvers over the bundled plugins.
// Unknown engines are an explicit error; there is no silent fallback.
func resolvers() (func(string) (d2graph.LayoutGraph, error), func(string) (d2graph.RouteEdges, error)) {
	plugins := enginePlugins()
	find := func(engine string) (d2plugin.Plugin, error) {
		p, ok := plugins[strings.ToLower(engine)]
		if !ok {
			ids := make([]string, 0, len(engineOrder))
			for _, e := range engineOrder {
				ids = append(ids, e.ID)
			}
			return nil, fmt.Errorf("layout engine %q is not available; supported engines: %s", engine, strings.Join(ids, ", "))
		}
		return p, nil
	}
	layout := func(engine string) (d2graph.LayoutGraph, error) {
		p, err := find(engine)
		if err != nil {
			return nil, err
		}
		return p.Layout, nil
	}
	router := func(engine string) (d2graph.RouteEdges, error) {
		p, err := find(engine)
		if err != nil {
			return nil, err
		}
		info, err := p.Info(context.Background())
		if err != nil {
			return nil, err
		}
		for _, f := range info.Features {
			if f == d2plugin.ROUTES_EDGES {
				rp, ok := p.(d2plugin.RoutingPlugin)
				if !ok {
					return nil, fmt.Errorf("layout engine %q advertises edge routing but does not implement it", engine)
				}
				return rp.RouteEdges, nil
			}
		}
		return nil, nil
	}
	return layout, router
}

func lower(s string) string { return strings.ToLower(s) }

var dagreOnce sync.Once

// initDagre gives the bundled Dagre plugin D2's default options. The plugin
// starts with nil options and the D2 CLI always hydrates it from flag
// defaults; without this, Layout dereferences nil.
func initDagre() {
	dagreOnce.Do(func() {
		b, err := json.Marshal(d2dagrelayout.DefaultOpts)
		if err == nil {
			err = d2plugin.DagrePlugin.HydrateOpts(b)
		}
		if err != nil {
			panic(fmt.Sprintf("compiler: initialize dagre defaults: %v", err))
		}
	})
}
