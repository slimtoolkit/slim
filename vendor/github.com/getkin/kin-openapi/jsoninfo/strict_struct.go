package jsoninfo

type StrictStruct interface {
	EncodeWith(encoder *ObjectEncoder, value interface{}) error
	DecodeWith(decoder *ObjectDecoder, value interface{}) error
}
