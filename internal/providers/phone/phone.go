package phone

import "strings"

func Normalize(raw, countryISO2, countryCallingCode string) (e164, national string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ""
	}
	value = strings.TrimPrefix(value, "+")
	code := strings.TrimPrefix(strings.TrimSpace(countryCallingCode), "+")
	national = value
	if code != "" && strings.HasPrefix(value, code) {
		national = strings.TrimPrefix(value, code)
	}
	if code != "" {
		e164 = "+" + code + national
	} else if strings.HasPrefix(raw, "+") {
		e164 = raw
	} else {
		e164 = value
	}
	return e164, national
}
