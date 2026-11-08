package design

import (
	"mygo-app/internal/design/components"

	"github.com/egoist/mygo/ui"
)

// TextField is a labelled single-line input.
func TextField(c *ui.Context, label string, v *string, placeholder string) {
	ui.Field(c, label, func() {
		ui.TextInput(c, v).Placeholder(placeholder)
	})
}

// TextAreaField is a labelled multi-line input.
func TextAreaField(c *ui.Context, label string, v *string, placeholder string) {
	ui.Field(c, label, func() {
		ui.TextArea(c, v).Placeholder(placeholder).Height(72)
	})
}

// IntField is a labelled whole number.
func IntField(c *ui.Context, label string, v *int, lo, hi float64) {
	cur := float64(*v)
	ui.Field(c, label, func() {
		ui.NumberInput(c, &cur, lo, hi, 1)
	})
	*v = int(cur)
}

// MoneyField edits an amount of cents as 元.
func MoneyField(c *ui.Context, label string, cents *int64) {
	cur := Yuan(*cents)
	ui.Field(c, label, func() {
		ui.NumberInput(c, &cur, 0, 1e9, 100)
	})
	*cents = Cents(cur)
}

// SelectField picks one of options, writing the chosen one to v.
func SelectField(c *ui.Context, label string, v *string, options []string) {
	if len(options) == 0 {
		return
	}
	ui.Field(c, label, func() {
		ui.Select(c, v, options)
	})
}

// SwitchField is a labelled on/off switch using SmoothSwitch.
func SwitchField(c *ui.Context, label string, on *bool) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		components.SmoothSwitch(c, on)
		ui.Text(c, label).FontSize(13)
	})
}

// FormActions is the row of buttons at the bottom of a form using modern ActionButton.
func FormActions(c *ui.Context, onCancel, onSave func(), saveLabel string) {
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if onCancel != nil && components.ActionButton(c, "取消", components.ButtonSecondary).Clicked() {
			onCancel()
		}
		if onSave != nil && components.ActionButton(c, saveLabel, components.ButtonPrimary).Clicked() {
			onSave()
		}
	})
}

// FormError shows what stops a form from saving.
func FormError(c *ui.Context, msg string) {
	if msg == "" {
		return
	}
	t := Theme(c)
	ui.Text(c, msg).FontSize(12).TextColor(t.Danger)
}

// Confirm asks before a destructive action, and reports whether the user
// confirmed it in this frame. The dialog closes itself either way.
func Confirm(c *ui.Context, open *bool, title, message, confirmLabel string) bool {
	return ui.AlertDialog(c, open, title, message, "取消", confirmLabel) == 1
}
