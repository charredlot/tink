// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
////////////////////////////////////////////////////////////////////////////////

package hybrid

import (
	"fmt"
	"io"

	"github.com/google/tink/go/core/cryptofmt"
	"github.com/google/tink/go/core/primitiveset"
	"github.com/google/tink/go/core/registry"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/tink"
)

// NewHybridDecrypt returns an HybridDecrypt primitive from the given keyset handle.
func NewHybridDecrypt(h *keyset.Handle) (tink.HybridDecrypt, error) {
	return NewHybridDecryptWithKeyManager(h, nil /*keyManager*/)
}

// NewHybridDecryptWithKeyManager returns an HybridDecrypt primitive from the given keyset handle
// and custom key manager.
// Deprecated: register the KeyManager and use New above.
func NewHybridDecryptWithKeyManager(h *keyset.Handle, km registry.KeyManager) (tink.HybridDecrypt, error) {
	ps, err := h.PrimitivesWithKeyManager(km)
	if err != nil {
		return nil, fmt.Errorf("hybrid_factory: cannot obtain primitive set: %s", err)
	}

	return newWrappedHybridDecrypt(ps)
}

// wrappedHybridDecrypt is an HybridDecrypt implementation that uses the underlying primitive set
// for decryption.
type wrappedHybridDecrypt struct {
	ps *primitiveset.PrimitiveSet
}

func newWrappedHybridDecrypt(ps *primitiveset.PrimitiveSet) (*wrappedHybridDecrypt, error) {
	if _, ok := (ps.Primary.Primitive).(tink.HybridDecrypt); !ok {
		return nil, fmt.Errorf("hybrid_factory: not a HybridDecrypt primitive")
	}

	for _, primitives := range ps.Entries {
		for _, p := range primitives {
			if _, ok := (p.Primitive).(tink.HybridDecrypt); !ok {
				return nil, fmt.Errorf("hybrid_factory: not a HybridDecrypt primitive")
			}
		}
	}

	ret := new(wrappedHybridDecrypt)
	ret.ps = ps

	return ret, nil
}

// Decrypt decrypts the given ciphertext and authenticates it with the given
// additional authenticated data. It returns the corresponding plaintext if the
// ciphertext is authenticated.
func (a *wrappedHybridDecrypt) Decrypt(ct, ad []byte) ([]byte, error) {
	// try non-raw keys
	prefixSize := cryptofmt.NonRawPrefixSize
	if len(ct) > prefixSize {
		prefix := ct[:prefixSize]
		ctNoPrefix := ct[prefixSize:]
		entries, err := a.ps.EntriesForPrefix(string(prefix))
		if err == nil {
			for i := 0; i < len(entries); i++ {
				p, ok := (entries[i].Primitive).(tink.HybridDecrypt)
				if !ok {
					return nil, fmt.Errorf("hybrid_factory: not a HybridDecrypt primitive")
				}

				pt, err := p.Decrypt(ctNoPrefix, ad)
				if err == nil {
					return pt, nil
				}
			}
		}
	}

	// try raw keys
	entries, err := a.ps.RawEntries()
	if err == nil {
		for i := 0; i < len(entries); i++ {
			p, ok := (entries[i].Primitive).(tink.HybridDecrypt)
			if !ok {
				return nil, fmt.Errorf("hybrid_factory: not a HybridDecrypt primitive")
			}

			pt, err := p.Decrypt(ct, ad)
			if err == nil {
				return pt, nil
			}
		}
	}

	// nothing worked
	return nil, fmt.Errorf("hybrid_factory: decryption failed")
}

type StreamingHybridDecrypt interface {
	NewDecryptingReader(r io.Reader, contextInfo []byte) (io.Reader, error)
}

func NewStreamingHybridDecrypt(h *keyset.Handle) (StreamingHybridDecrypt, error) {
	return NewStreamingHybridDecryptWithKeyManager(h, nil /*keyManager*/)
}

func NewStreamingHybridDecryptWithKeyManager(h *keyset.Handle, km registry.KeyManager) (StreamingHybridDecrypt, error) {
	ps, err := h.PrimitivesWithKeyManager(km)
	if err != nil {
		return nil, fmt.Errorf("hybrid_factory: cannot obtain primitive set: %s", err)
	}

	return newStreamingHybridDecryptPrimitiveSet(ps)
}

type wrappedStreamingHybridDecrypt struct {
	ps *primitiveset.PrimitiveSet
}

func newStreamingHybridDecryptPrimitiveSet(ps *primitiveset.PrimitiveSet) (*wrappedStreamingHybridDecrypt, error) {
	if _, ok := (ps.Primary.Primitive).(StreamingHybridDecrypt); !ok {
		return nil, fmt.Errorf("hybrid_factory: primary %T is not a StreamingHybridDecrypt primitive", ps.Primary.Primitive)
	}

	for _, primitives := range ps.Entries {
		for _, p := range primitives {
			if _, ok := (p.Primitive).(StreamingHybridDecrypt); !ok {
				return nil, fmt.Errorf("hybrid_factory: entry %T is not a StreamingHybridDecrypt primitive", p.Primitive)
			}
		}
	}

	ret := new(wrappedStreamingHybridDecrypt)
	ret.ps = ps

	return ret, nil
}

func (wshd *wrappedStreamingHybridDecrypt) NewDecryptingReader(w io.Reader, contextInfo []byte) (io.Reader, error) {
	primary := wshd.ps.Primary
	p, ok := (primary.Primitive).(StreamingHybridDecrypt)
	if !ok {
		return nil, fmt.Errorf("streaminghybrid_factory.go: %T is not a StreamingHybridDecrypt primitive", primary.Primitive)
	}

	return p.NewDecryptingReader(w, contextInfo)
}
