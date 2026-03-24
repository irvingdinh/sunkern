package http

import (
	"fmt"
	"io"
	"mime/multipart"
	httpstd "net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// DefaultMaxMemory is the default maximum bytes stored in memory when
// parsing a multipart form. Files exceeding this threshold are written
// to temporary files on disk. Defaults to 32 MiB.
const DefaultMaxMemory int64 = 32 << 20

// FileConstraint defines validation rules for an uploaded file.
type FileConstraint struct {
	// MaxSize is the maximum allowed file size in bytes. Zero means no limit.
	MaxSize int64

	// AllowedTypes is the set of permitted MIME content types (e.g.,
	// "image/png", "image/jpeg", "application/pdf"). An empty slice
	// allows all types. Matching is case-insensitive and checks the
	// Content-Type header set by the client. For stricter validation
	// that doesn't trust the client, use DetectFileType.
	AllowedTypes []string
}

var (
	fileHeaderPtrType   = reflect.TypeOf((*multipart.FileHeader)(nil))
	fileHeaderSliceType = reflect.TypeOf(([]*multipart.FileHeader)(nil))
)

// FormFile returns the first uploaded file for the named form field.
// Parses the multipart form lazily with DefaultMaxMemory if needed.
// Returns ErrBadRequest if the request is not multipart or the field
// is missing.
func FormFile(r *httpstd.Request, name string) (*multipart.FileHeader, error) {
	if err := parseMultipart(r); err != nil {
		return nil, err
	}
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, ErrBadRequest.WithMessage(fmt.Sprintf("Missing file field %q", name))
	}
	files := r.MultipartForm.File[name]
	if len(files) == 0 {
		return nil, ErrBadRequest.WithMessage(fmt.Sprintf("Missing file field %q", name))
	}
	return files[0], nil
}

// FormFiles returns all uploaded files for the named form field.
// Parses the multipart form lazily with DefaultMaxMemory if needed.
// Returns ErrBadRequest if the request is not multipart or the field
// is missing.
func FormFiles(r *httpstd.Request, name string) ([]*multipart.FileHeader, error) {
	if err := parseMultipart(r); err != nil {
		return nil, err
	}
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, ErrBadRequest.WithMessage(fmt.Sprintf("Missing file field %q", name))
	}
	files := r.MultipartForm.File[name]
	if len(files) == 0 {
		return nil, ErrBadRequest.WithMessage(fmt.Sprintf("Missing file field %q", name))
	}
	return files, nil
}

// SaveFile saves an uploaded file to dst, creating parent directories
// as needed with 0o755 permissions. Returns an error if the file cannot
// be read or the destination cannot be written.
func SaveFile(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return fmt.Errorf("upload: open source: %w", err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("upload: create directory: %w", err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("upload: create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("upload: write file: %w", err)
	}
	return nil
}

// ValidateFile checks an uploaded file against the given constraints.
// Size is checked against FileHeader.Size. Content type is checked
// against the Content-Type header set by the client (case-insensitive).
//
// For stricter type checking that inspects actual file bytes rather
// than trusting the client header, use DetectFileType first and pass
// the result to your own allow-list check.
//
// Returns ErrPayloadTooLarge if the file exceeds MaxSize.
// Returns ErrUnsupportedMediaType if the content type is not allowed.
func ValidateFile(fh *multipart.FileHeader, c FileConstraint) error {
	if c.MaxSize > 0 && fh.Size > c.MaxSize {
		return ErrPayloadTooLarge.WithMessage(
			fmt.Sprintf("File size %d bytes exceeds limit of %d bytes", fh.Size, c.MaxSize))
	}
	if len(c.AllowedTypes) > 0 {
		ct := fh.Header.Get("Content-Type")
		allowed := false
		for _, t := range c.AllowedTypes {
			if strings.EqualFold(ct, t) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrUnsupportedMediaType.WithMessage(
				fmt.Sprintf("File type %q not allowed, accepted: %s", ct, strings.Join(c.AllowedTypes, ", ")))
		}
	}
	return nil
}

// DetectFileType reads the first 512 bytes of the uploaded file to
// determine its actual content type using Go's http.DetectContentType.
// This is more reliable than the Content-Type header from the client
// and should be preferred for user-facing uploads where security matters.
// Returns "application/octet-stream" if the file is empty or detection
// is inconclusive.
func DetectFileType(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("upload: open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream", nil
	}
	return httpstd.DetectContentType(buf[:n]), nil
}

// BindForm parses multipart form data into dst using `form` struct tags.
// Supports text fields (string, int, int64, uint, uint64, float64, bool)
// and file fields (*multipart.FileHeader, []*multipart.FileHeader).
// Runs validation using `validate` struct tags after binding.
//
// Returns ErrBadRequest if the form cannot be parsed or a field value
// is invalid. Returns ErrValidation with field-level details if
// validation fails. dst must be a pointer to a struct.
//
//	type UploadReq struct {
//	    Title string                `form:"title" validate:"required,max=200"`
//	    File  *multipart.FileHeader `form:"file"  validate:"required"`
//	}
func BindForm(r *httpstd.Request, dst any) error {
	if err := parseMultipart(r); err != nil {
		return err
	}

	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return ErrInternal.WithMessage("BindForm: dst must be a pointer to a struct")
	}

	if err := bindFormFields(rv.Elem(), r); err != nil {
		return err
	}
	return Validate(dst)
}

func bindFormFields(rv reflect.Value, r *httpstd.Request) error {
	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		// Recurse into embedded structs.
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if err := bindFormFields(rv.Field(i), r); err != nil {
				return err
			}
			continue
		}

		tag := field.Tag.Get("form")
		if tag == "" || tag == "-" {
			continue
		}

		fv := rv.Field(i)

		// Handle *multipart.FileHeader fields.
		if field.Type == fileHeaderPtrType {
			if r.MultipartForm != nil && r.MultipartForm.File != nil {
				if files := r.MultipartForm.File[tag]; len(files) > 0 {
					fv.Set(reflect.ValueOf(files[0]))
				}
			}
			continue
		}

		// Handle []*multipart.FileHeader fields.
		if field.Type == fileHeaderSliceType {
			if r.MultipartForm != nil && r.MultipartForm.File != nil {
				if files := r.MultipartForm.File[tag]; len(files) > 0 {
					fv.Set(reflect.ValueOf(files))
				}
			}
			continue
		}

		// Handle text form fields.
		val := r.FormValue(tag)
		if val == "" {
			continue
		}
		if err := setFieldFromString(fv, val); err != nil {
			return ErrBadRequest.WithMessage(fmt.Sprintf("Invalid form field %q: %s", tag, err.Error()))
		}
	}
	return nil
}

func parseMultipart(r *httpstd.Request) error {
	if r.MultipartForm != nil {
		return nil
	}
	if err := r.ParseMultipartForm(DefaultMaxMemory); err != nil {
		if isMaxBytesError(err) {
			return ErrPayloadTooLarge
		}
		return ErrBadRequest.WithMessage("Invalid multipart form data")
	}
	return nil
}
