package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/i18n"
)

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		name     string
		minor    int64
		decimals int
		currency string
		locale   i18n.Locale
		want     string
	}{
		{"dollars", 1250, 2, "USD", i18n.EN, "$12.50"},
		{"a float's worth of cents", 14157, 2, "USD", i18n.EN, "$141.57"},
		{"grouped thousands", 123456789, 2, "USD", i18n.EN, "$1,234,567.89"},
		{"under a dollar keeps its zero", 5, 2, "USD", i18n.EN, "$0.05"},
		{"zero", 0, 2, "USD", i18n.EN, "$0.00"},
		{"the sign stays outside the symbol", -1250, 2, "USD", i18n.EN, "-$12.50"},
		{"a currency with no decimals", 5000, 0, "JPY", i18n.EN, "¥5,000"},
		{"three decimals", 12345, 3, "USD", i18n.EN, "$12.345"},
		{"a lowercase code still finds its symbol", 100, 2, "usd", i18n.EN, "$1.00"},
		{"no symbol falls back to the code", 123400, 2, "CHF", i18n.EN, "CHF" + nbsp + "1,234.00"},
		{"no currency is a bare number", 1250, 2, "", i18n.EN, "12.50"},
		{"portuguese: comma, dot grouping, symbol first", 123456, 2, "USD", i18n.PtBR, "$" + nbsp + "1.234,56"},
		{"portuguese real", 1250, 2, "BRL", i18n.PtBR, "R$" + nbsp + "12,50"},
		{"spanish puts the symbol after", 1250, 2, "EUR", i18n.ES, "12,50" + nbsp + "€"},
		{"spanish code goes after too", 123400, 2, "CHF", i18n.ES, "1.234,00" + nbsp + "CHF"},
		{"russian groups with a no-break space", 123456, 2, "USD", i18n.RU, "1" + nbsp + "234,56" + nbsp + "$"},
		{"negative in russian", -50, 2, "EUR", i18n.RU, "-0,50" + nbsp + "€"},
		{"japanese is the english shape", 1250, 2, "USD", i18n.JA, "$12.50"},
	}
	for _, c := range cases {
		if got := formatMoney(c.minor, c.decimals, c.currency, c.locale); got != c.want {
			t.Errorf("%s: formatMoney(%d, %d, %q, %s) = %q, want %q", c.name, c.minor, c.decimals, c.currency, c.locale, got, c.want)
		}
	}
}
