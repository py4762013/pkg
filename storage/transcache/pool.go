package transcache

import (
	"bytes"
	"io"
	"io/ioutil"
	"sync"
)

type pooledCodec struct {
	encoderPool sync.Pool
	decoderPool sync.Pool
}

// NewPooledCodec delegates to the passed codec for creating Encoders/Decoders.
// Newly created Encoder/Decoders will Encode/Decode the passed sample structs
// without actually writing/reading from their respective Writer/Readers. This
// is useful for Codec's like GobCodec{} which encodes/decodes extra type
// information whenever it sees a new type. Pass sample values for types you
// plan on Encoding/Decoding to this method in order to avoid the storage
// overhead of encoding their type information for every NewEncoder/NewDecoder.
// Make sure you use gob.Register() for every type you plan to use otherwise
// there will be errors. Setting the types causes a priming of the encoder and
// decoder each type an encoder or a decoder object will be returned from the
// pool. This function panics if the types, used for priming, can neither be
// encoded nor decoded.
func NewPooledCodec(codec Codecer, types ...interface{}) Codecer {
	return &pooledCodec{
		encoderPool: sync.Pool{New: func() interface{} {
			var enc delegateEncoder
			enc.Encoder = codec.NewEncoder(&enc)
			if len(types) > 0 {
				enc.Writer = ioutil.Discard
				if err := enc.Encode(types); err != nil {
					panic(err)
				}
				enc.Writer = nil
			}
			return &enc
		}},
		decoderPool: sync.Pool{New: func() interface{} {
			var dec delegateDecoder
			dec.Decoder = codec.NewDecoder(&dec)
			if len(types) > 0 {
				var buf bytes.Buffer
				enc := codec.NewEncoder(&buf)
				if err := enc.Encode(types); err != nil {
					panic(err)
				}
				var testTypes []interface{}
				dec.Reader = bytes.NewReader(buf.Bytes())
				if err := dec.Decode(&testTypes); err != nil {
					panic(err)
				}
				dec.Reader = nil
			}
			return &dec
		}},
	}
}

func (p *pooledCodec) NewEncoder(w io.Writer) Encoder {
	enc := p.encoderPool.Get().(*delegateEncoder)
	enc.Writer = w
	return enc
}

func (p *pooledCodec) NewDecoder(r io.Reader) Decoder {
	dec := p.decoderPool.Get().(*delegateDecoder)
	dec.Reader = r
	return dec
}

func (p *pooledCodec) PutEncoder(enc Encoder) {
	p.encoderPool.Put(enc)
}

func (p *pooledCodec) PutDecoder(dec Decoder) {
	p.decoderPool.Put(dec)
}

type delegateEncoder struct {
	Encoder
	io.Writer
}

type delegateDecoder struct {
	Decoder
	io.Reader
}