package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
)

// nbsp keeps an amount and its symbol or code on one line.
const nbsp = " "

// currencySymbols is every currency the window has a symbol for; any other
// shows its ISO 4217 code.
//
// shortcut: seven currencies, the ones the UI fonts draw without a fallback; a
// provider billing in another reads as its code, never as a wrong symbol.
var currencySymbols = map[string]string{
	"USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥", "BRL": "R$",
}

// moneyStyle is how a locale writes an amount: its separators and which side
// of the number the currency sits on.
type moneyStyle struct {
	decimal, group string
	suffix         bool
	// gap separates a symbol from the number; a code is always separated.
	gap string
}

func moneyStyleOf(l i18n.Locale) moneyStyle {
	switch l {
	case i18n.PtBR:
		return moneyStyle{decimal: ",", group: ".", gap: nbsp}
	case i18n.ES:
		return moneyStyle{decimal: ",", group: ".", suffix: true, gap: nbsp}
	case i18n.RU:
		return moneyStyle{decimal: ",", group: nbsp, suffix: true, gap: nbsp}
	}
	return moneyStyle{decimal: ".", group: ","}
}

// formatMoney writes minor units (cents) as an amount in l: "$12.50",
// "12,50 €", "CHF 1,234.00". The sign leads the whole amount, outside the
// symbol; a currency with no symbol shows its code, and none at all shows the
// bare number. decimals is how many of the minor digits sit after the point.
func formatMoney(minor int64, decimals int, currency string, l i18n.Locale) string {
	style := moneyStyleOf(l)
	sign, abs := "", minor
	if minor < 0 {
		sign, abs = "-", -minor
	}
	scale := int64(1)
	for range decimals {
		scale *= 10
	}
	number := groupDigits(strconv.FormatInt(abs/scale, 10), style.group)
	if decimals > 0 {
		number += style.decimal + fmt.Sprintf("%0*d", decimals, abs%scale)
	}
	currency = strings.ToUpper(currency)
	switch symbol, known := currencySymbols[currency]; {
	case currency == "":
		return sign + number
	case known && style.suffix:
		return sign + number + style.gap + symbol
	case known:
		return sign + symbol + style.gap + number
	case style.suffix:
		return sign + number + nbsp + currency
	}
	return sign + currency + nbsp + number
}

// groupDigits puts sep between each thousand of a run of digits.
func groupDigits(digits, sep string) string {
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(d)
	}
	return b.String()
}
