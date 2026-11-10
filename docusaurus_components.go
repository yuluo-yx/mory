package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/blocks"
)

// Component choices are view state; toggling a preview never changes MDX bytes.
func (a *app) docusaurusComponent(c *ui.Context, n *blocks.Node, readOnly bool, depth int) {
	s := a.blockEditor
	if s.componentChoice == nil {
		s.componentChoice = map[string]string{}
	}
	children := func(node *blocks.Node) {
		for _, child := range blocks.Visible(node.Children) {
			a.blockNode(c, child, readOnly, depth+1)
		}
	}
	if n.Kind == "details" {
		label := n.Attr("summary")
		if label == "" {
			label = a.tr("Details")
		}
		open := s.componentChoice[n.ID] == "open"
		marker := "▸ "
		if open {
			marker = "▾ "
		}
		if insertMenuButton(c, marker+label).Label(a.tr("Toggle details ") + label).Clicked() {
			if open {
				s.componentChoice[n.ID] = "closed"
			} else {
				s.componentChoice[n.ID] = "open"
			}
		}
		if open {
			children(n)
		}
		return
	}
	if n.Kind == "tab-item" {
		children(n)
		return
	}
	key := n.ID
	if group := n.Attr("groupId"); group != "" {
		key = "group:" + group
	}
	choice := s.componentChoice[key]
	items := []*blocks.Node{}
	for _, child := range n.Children {
		if child.Kind == "tab-item" {
			items = append(items, child)
		}
	}
	if choice == "" {
		choice = n.Attr("defaultValue")
		if choice == "" && len(items) > 0 {
			choice = items[0].Attr("value")
			for _, item := range items {
				if item.Attr("default") == "true" {
					choice = item.Attr("value")
				}
			}
		}
	}
	ui.Row(c).Wrap().Gap(4).Children(func() {
		for _, item := range items {
			label := item.Attr("label")
			if label == "" {
				label = item.Attr("value")
			}
			b := ghostButton(c.Key(item.ID), label).Label(a.tr("Tab ") + label)
			if b.Hovered() || choice == item.Attr("value") {
				b.Background(c.Theme().Accent.Alpha(.12)).TextColor(c.Theme().Accent)
			}
			if b.Clicked() {
				s.componentChoice[key] = item.Attr("value")
			}
		}
	})
	for _, item := range items {
		if item.Attr("value") == choice {
			children(item)
		}
	}
}
