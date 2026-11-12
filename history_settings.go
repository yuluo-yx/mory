package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/yuluo-yx/mory/internal/library"
)

func (a *app) setHistorySettings(policy library.HistorySettings) {
	previous := a.lib.Settings.History
	a.lib.Settings.History = policy
	if !a.savePreferences() {
		a.lib.Settings.History = previous
	}
}

func (a *app) historySettings(c *ui.Context) {
	settingsCard(c, a.tr("Document history"), func() {
		policy := a.lib.HistoryPolicy()
		enabled := !policy.Disabled
		if ui.Checkbox(c, &enabled, a.tr("Keep version history")).Changed() {
			policy.Disabled = !enabled
			a.setHistorySettings(policy)
		}
		ui.Text(c, a.tr("Automatic saving stays on. Turning history off keeps existing versions and recovery copies.")).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
		for _, setting := range []struct {
			label   string
			control string
			value   *int
			options []string
		}{
			{"Snapshot interval (minutes)", "History snapshot interval", &policy.IntervalMinutes, []string{"1", "5", "10", "30", "60"}},
			{"Keep history (days)", "History retention period", &policy.RetentionDays, []string{"7", "30", "90", "365"}},
			{"Maximum versions per document", "History version limit", &policy.MaxVersions, []string{"20", "50", "100", "200", "500"}},
			{"History storage per document (MiB)", "History storage limit", &policy.MaxMiB, []string{"16", "32", "64", "128", "256"}},
		} {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.tr(setting.label)).Grow(1)
				value := strconv.Itoa(*setting.value)
				if ui.Select(c, &value, setting.options).Label(a.tr(setting.control)).Width(100).Disabled(policy.Disabled).Changed() {
					*setting.value, _ = strconv.Atoi(value)
					a.setHistorySettings(policy)
				}
			})
		}
		ui.Text(c, a.tr("Older versions are removed on the next document save or history view according to these limits. The latest version is always kept, even if it exceeds the limits.")).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
		if ghostButton(c, a.tr("Reset history settings")).AlignSelf(ui.Start).Disabled(policy == library.DefaultHistorySettings()).Clicked() {
			a.setHistorySettings(library.DefaultHistorySettings())
		}
	})
}
