package language

import (
	"bytes"
	"encoding/json"
	"strings"

	translation "github.com/K7mQ2vR9/language-pkg"
)

var messageKeys = map[string]string{
	"Authentication failed.":     "auth.failed",
	"User not login.":            "auth.not_logged_in",
	"No permission":              "auth.permission_denied",
	"Token invalid.":             "auth.token_invalid",
	"Internal Server Error":      "common.internal_error",
	"Request failed.":            "common.request_failed",
	"Service Unavailable":        "common.service_unavailable",
	"Status service unavailable": "common.status_service_unavailable",
	"Request decryption failed.": "request.decryption_failed",
	"Request forbid.":            "request.forbidden",
}

// TranslateMessage translates a response message or a dictionary key.
func TranslateMessage(message, locale string) string {
	key, ok := messageKeys[strings.TrimSpace(message)]
	if !ok {
		key = message
	}
	return translation.Translate(locale, key, message)
}

// SwitchLanguage translates matching string values when response data is encoded.
// Plain strings and byte slices keep their types for the AES response path.
func SwitchLanguage(data any, locale string) any {
	if locale == "" {
		return data
	}
	switch value := data.(type) {
	case nil, []byte:
		return data
	case string:
		return translation.Translate(locale, value, value)
	default:
		return localizedJSON{data: value, locale: locale}
	}
}

type localizedJSON struct {
	data   any
	locale string
}

func (data localizedJSON) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(data.data)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	rewrite := jsonTranslation{decoder: decoder, raw: raw, locale: data.locale}
	if err := rewrite.value(false); err != nil {
		return nil, err
	}
	if rewrite.output == nil {
		return raw, nil
	}
	return append(rewrite.output, raw[rewrite.copied:]...), nil
}

type jsonTranslation struct {
	decoder *json.Decoder
	raw     []byte
	locale  string
	output  []byte
	copied  int64
}

func (rewrite *jsonTranslation) value(protected bool) error {
	previous := rewrite.decoder.InputOffset()
	token, err := rewrite.decoder.Token()
	if err != nil {
		return err
	}
	switch token := token.(type) {
	case json.Delim:
		for rewrite.decoder.More() {
			protectedChild := false
			if token == '{' {
				key, err := rewrite.decoder.Token()
				if err != nil {
					return err
				}
				name := key.(string)
				protectedChild = strings.EqualFold(name, "id") || strings.EqualFold(name, "code")
			}
			if err := rewrite.value(protectedChild); err != nil {
				return err
			}
		}
		_, err = rewrite.decoder.Token()
		return err
	case string:
		if protected {
			return nil
		}
		text := translation.Translate(rewrite.locale, token, token)
		if text != token {
			end := rewrite.decoder.InputOffset()
			// The decoder may consume a comma or colon before the value.
			start := previous + int64(bytes.IndexByte(rewrite.raw[previous:end], '"'))
			encoded, err := json.Marshal(text)
			if err != nil {
				return err
			}
			rewrite.output = append(rewrite.output, rewrite.raw[rewrite.copied:start]...)
			rewrite.output = append(rewrite.output, encoded...)
			rewrite.copied = end
		}
	}
	return nil
}
