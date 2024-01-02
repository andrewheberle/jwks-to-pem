package jwks

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/kalafut/imohash"
)

type JWKS struct {
	keyset []JWK
}

type JWK struct {
	key  jwkset.JWK
	data []byte
}

var (
	// ErrPatternNotParsed is returned when the provided file name pattern
	// could not be parsed
	ErrPatternNotParsed = errors.New("pattern could not be parsed")

	// ErrUnsupportedAlgorithm is returned when the JWKS contains a key
	// type that is not RSA or ECDSA
	ErrUnsupportedAlgorithm = errors.New("unsupported key algorithm")

	// ErrNotRSAPublicKey is returned when the key could not be
	// converted to a *rsa.PublicKey despite the JWK specifying the
	// algorithm as RS256, RS384 or RS512
	ErrNotRSAPublicKey = errors.New("was not a RSA public key")

	// ErrNotECDSAPublicKey is returned when the key could not be
	// converted to a *ecdsa.PublicKey despite the JWK specifying the
	// algorithm as ES256, ES384 or ES512
	ErrNotECDSAPublicKey = errors.New("was not a ECDSA public key")

	// ErrPEMEncodeFailed is returned when the public key could not
	// be encoded into PEM format.
	ErrPEMEncodeFailed = errors.New("was not a RSA public key")

	// ErrWriteFailed is returned when the public key could not
	// written.
	ErrWriteFailed = errors.New("could not write public key")

	// ErrTemplateProblem is returned when the key filename
	// pattern could not be templated correctly.
	ErrTemplateProblem = errors.New("problem executing template")
)

type WriteError struct {
	Message string
	KeyID   string
	Err     error
}

func (e *WriteError) Error() string {
	if e.KeyID != "" {
		return e.Message + "(KID: " + e.KeyID + "): " + e.Err.Error()
	}

	return e.Message + ": " + e.Err.Error()
}

func (e *WriteError) Unwrap() error { return e.Err }

func GetJWKS(url string, timeout time.Duration) (*JWKS, error) {
	// set up http client to grab jwks
	j, err := jwkset.NewDefaultHTTPClient([]string{url})
	if err != nil {
		return nil, err
	}

	// only wait for timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// read keys from jwks
	keys, err := j.KeyReadAll(ctx)
	if err != nil {
		return nil, err
	}

	keyset := new(JWKS)
	for _, key := range keys {
		keyset.keyset = append(keyset.keyset, JWK{key: key})
	}

	return keyset, nil
}

func (j *JWKS) WriteKeys(pattern, output string) (bool, error) {
	var err error
	var keyChanged bool

	// set up template
	t, err := template.New("pattern").Parse(pattern)
	if err != nil {
		return keyChanged, &WriteError{Message: "pattern could not be parsed", Err: err}
	}

	// keep track of errors
	errs := make([]error, 0)

	// iterate over keys
	for n, jwk := range j.keyset {
		// grab key id
		keyID := jwk.KID()

		data, err := jwk.PEM()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// write to stdout if no output is provided
		if output == "" {
			os.Stderr.Write(data)
			continue
		}

		// execute template as string
		name := new(bytes.Buffer)
		if err := t.Execute(name, struct {
			Index int
			KeyID string
		}{
			Index: n,
			KeyID: keyID,
		}); err != nil {
			errs = append(errs, &WriteError{Message: "template execution failed", KeyID: keyID, Err: err})
			continue
		}

		// build output file
		outFile := filepath.Join(output, name.String())

		// check if any changes have occurred
		if changed, err := jwk.Changed(outFile); err != nil {
			errs = append(errs, &WriteError{Message: "error comparing keys", KeyID: keyID, Err: err})
			slog.Debug("error comparing keys", "error", err)
			continue
		} else if !changed {
			continue
		}

		// write out pem encoded file
		if err := jwk.Write(outFile); err != nil {
			errs = append(errs, &WriteError{Message: "writing key failed", KeyID: keyID, Err: err})
			continue
		}

		// on successful write set keyChanged to "true"
		keyChanged = true
	}

	// return any errors
	return keyChanged, errors.Join(errs...)
}

func (k *JWK) ALG() string {
	return k.key.Marshal().ALG.String()
}

func (k *JWK) KID() string {
	return k.key.Marshal().KID
}

func (jwk *JWK) Bytes() ([]byte, error) {
	// check if this is already done
	if jwk.data != nil {
		return jwk.data, nil
	}

	var (
		data []byte
		err  error
	)

	// convert key to byte slice ready to encode into PEM format
	switch jwk.ALG() {
	case "RS256", "RS384", "RS512":
		k, ok := jwk.key.Key().(*rsa.PublicKey)
		if !ok {
			return nil, &WriteError{Message: "invalid key", KeyID: jwk.KID(), Err: ErrNotRSAPublicKey}
		}

		data, err = x509.MarshalPKIXPublicKey(k)
	case "ES256", "ES384", "ES512":
		k, ok := jwk.key.Key().(*ecdsa.PublicKey)
		if !ok {
			return nil, &WriteError{Message: "invalid key", KeyID: jwk.KID(), Err: ErrNotECDSAPublicKey}
		}

		data, err = x509.MarshalPKIXPublicKey(k)
	default:
		return nil, &WriteError{Message: "invalid key", KeyID: jwk.KID(), Err: ErrUnsupportedAlgorithm}
	}

	// cache data for later
	if err == nil {
		jwk.data = data
	}

	return data, err
}

func (jwk *JWK) Changed(current string) (bool, error) {
	// get key as PEM encoded byte slice
	data, err := jwk.PEM()
	if err != nil {
		return false, err
	}

	// compare hashes and return result
	return keychanged(current, data)
}

func keychanged(current string, data []byte) (bool, error) {
	// use fast hashing function
	hasher := imohash.New()
	currenthash, err := hasher.SumFile(current)
	if err != nil {
		// check error was not just due to missing file
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
	}
	a, b := hasher.Sum(data), currenthash

	bdata, _ := os.ReadFile(current)

	slog.Debug("comparison results", "a-hash", a, "b-hash", b, "a-data", data, "b-data", bdata)

	// compare hashes and return result
	return !bytes.Equal(a[:], b[:]), nil
}

func (jwk *JWK) PEM() ([]byte, error) {
	// grab as byte slice
	b, err := jwk.Bytes()
	if err != nil {
		return nil, err
	}

	// encode pem version to "buf"
	buf := new(bytes.Buffer)
	if err := pem.Encode(buf, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: b,
	}); err != nil {
		return nil, &WriteError{Message: "could not encode to PEM format", KeyID: jwk.KID(), Err: err}
	}

	// return data as []byte
	return buf.Bytes(), nil

}

func (jwk *JWK) Write(name string) error {
	// grab as PEM encoded byte slice
	data, err := jwk.PEM()
	if err != nil {
		return err
	}

	// create temp file
	f, err := os.CreateTemp(filepath.Dir(name), "key*")
	if err != nil {
		return err
	}

	// save temp file name
	tempName := f.Name()

	// close temp file and remove once done
	defer func() {
		f.Close()
		os.Remove(tempName)
	}()

	// write data to temp file
	if _, err := f.Write(data); err != nil {
		return err
	}

	// close temp file
	if err := f.Close(); err != nil {
		return err
	}

	// move into place
	return os.Rename(tempName, name)
}
