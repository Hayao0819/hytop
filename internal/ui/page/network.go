package page

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/netgraph"
	"github.com/Hayao0819/hytop/internal/ui/widget/statgrid"
)

const maxCards = 4

// Network stacks one mirrored plot per card: receive above the line, send
// below, with the running totals and the negotiated link rate on its heading.
type networkPage struct {
	reactea.Wrapper

	env   Env
	box   *layout.Box
	cards []string
}

func Network(env Env) reactea.Component {
	p := &networkPage{env: env}

	p.box = layout.Column(p.items(env.Interfaces())...)
	p.Wrapper = reactea.Wrap(p.box)

	return p
}

// Update rebuilds panes after interface discovery or hot-plug changes.
func (p *networkPage) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if cards := p.env.Interfaces(); !slices.Equal(cards, p.cards) {
		p.box.SetItems(p.items(cards)...)
	}

	return p.Wrapper.Update(ctx, msg)
}

func (p *networkPage) items(cards []string) []layout.Item {
	env := p.env
	p.cards = slices.Clone(cards)

	items := []layout.Item{
		layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
			return lipgloss.NewStyle().Bold(true).Width(ctx.Width()).Render("Network")
		})),
	}

	// Six rows are the minimum useful plot height.
	const perCard = 6

	shown := cards
	if len(shown) > maxCards {
		shown = shown[:maxCards]
	}

	for _, iface := range shown {
		card := netgraph.New(env.Store, env.Caps, iface, env.Theme.Colour(theme.NetRx), env.Theme.Colour(theme.NetTx))
		card.Span = env.Span

		items = append(items, layout.Grow(1, layout.Framed(pane, card)).Bounds(perCard, 0))
	}

	switch {
	case len(cards) == 0:
		items = append(items, layout.Grow(1, reactea.Text("no interfaces with a link")))
	case len(cards) > len(shown):
		hidden := len(cards) - len(shown)

		items = append(items, layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
			return lipgloss.NewStyle().Faint(true).Width(ctx.Width()).
				Render(fmt.Sprintf(" %d quieter interfaces not shown", hidden))
		})))
	}

	items = append(items, layout.Fixed(2, statgrid.New(env.Store,
		statgrid.Stat{Label: "Receive", Key: "net.total.rx", Unit: series.BytesPerSecond, Precision: 1, Big: true},
		statgrid.Stat{Label: "Send", Key: "net.total.tx", Unit: series.BytesPerSecond, Precision: 1, Big: true},
		statgrid.Stat{Label: "Received", Key: "net.total.rx.total", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "Sent", Key: "net.total.tx.total", Unit: series.Bytes, Precision: 1},
	)))

	return items
}
