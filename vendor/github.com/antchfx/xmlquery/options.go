package xmlquery

import (
	"encoding/xml"
	"io"
)

type ParserOptions struct {
	Decoder *DecoderOptions
}

func (options ParserOptions) apply(parser *parser) {
	if options.Decoder != nil {
		(*options.Decoder).apply(parser.decoder)
	}
}

// DecoderOptions implement the very same options than the standard
// encoding/xml package. Please refer to this documentation:
// https://golang.org/pkg/encoding/xml/#Decoder
type DecoderOptions struct {
	Strict        bool
	AutoClose     []string
	Entity        map[string]string
	CharsetReader func(charset string, input io.Reader) (io.Reader, error)
}

func (options DecoderOptions) apply(decoder *xml.Decoder) {
	decoder.Strict = options.Strict
	decoder.AutoClose = options.AutoClose
	decoder.Entity = options.Entity
	decoder.CharsetReader = options.CharsetReader
}
