package otp

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

const (
	maxMigrationBytes   = 64 << 10
	maxMigrationPages   = 64
	maxMigrationRecords = 1000
)

var (
	ErrMigration      = errors.New("invalid or unsupported Google Authenticator export")
	ErrMigrationBatch = errors.New("export pages belong to different or conflicting batches")
)

type MigrationRecord struct {
	Name        string
	Issuer      string
	URI         string
	Unsupported string
}

type MigrationPage struct {
	Version int
	Size    int
	Index   int
	ID      int32
	Records []MigrationRecord
	digest  [32]byte
}

// Migration fields follow the independently documented Google export protobuf.
// Wire parsing uses protowire; errors deliberately exclude payload contents.
func ParseMigration(value string) (MigrationPage, error) {
	if len(value) > maxMigrationBytes*2 {
		return MigrationPage{}, ErrMigration
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "otpauth-migration" || u.Host != "offline" || u.User != nil || (u.Path != "" && u.Path != "/") || u.Opaque != "" || strings.Contains(value, "#") {
		return MigrationPage{}, ErrMigration
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q) != 1 || len(q["data"]) != 1 || q.Get("data") == "" {
		return MigrationPage{}, ErrMigration
	}
	encoded := q.Get("data")
	encoding := base64.StdEncoding
	if !strings.Contains(encoded, "=") {
		encoding = base64.RawStdEncoding
	}
	data, err := encoding.Strict().DecodeString(encoded)
	defer clear(data)
	if err != nil || len(data) > maxMigrationBytes {
		return MigrationPage{}, ErrMigration
	}
	page := MigrationPage{digest: sha256.Sum256(data)}
	err = walkMigration(data, func(number protowire.Number, kind protowire.Type, field []byte) error {
		if number == 1 {
			if kind != protowire.BytesType || len(page.Records) >= maxMigrationRecords {
				return ErrMigration
			}
			record, err := parseMigrationRecord(field)
			if err != nil {
				return err
			}
			page.Records = append(page.Records, record)
			return nil
		}
		if number < 2 || number > 5 {
			return nil
		}
		if kind != protowire.VarintType {
			return ErrMigration
		}
		n, _ := protowire.ConsumeVarint(field)
		if n > uint64(^uint32(0)) {
			return ErrMigration
		}
		switch number {
		case 2:
			page.Version = int(n)
		case 3:
			page.Size = int(n)
		case 4:
			page.Index = int(n)
		case 5:
			page.ID = int32(n)
		}
		return nil
	})
	if err != nil || !validMigrationPage(page) {
		return MigrationPage{}, ErrMigration
	}
	return page, nil
}

func walkMigration(data []byte, consume func(protowire.Number, protowire.Type, []byte) error) error {
	var seen uint8
	for len(data) > 0 {
		number, kind, n := protowire.ConsumeTag(data)
		if n < 0 || number <= 0 || kind == protowire.StartGroupType || kind == protowire.EndGroupType {
			return ErrMigration
		}
		data = data[n:]
		length := protowire.ConsumeFieldValue(number, kind, data)
		if length < 0 {
			return ErrMigration
		}
		field := data[:length]
		if kind == protowire.BytesType {
			var count int
			field, count = protowire.ConsumeBytes(field)
			if count < 0 {
				return ErrMigration
			}
		}
		// Only payload field1 repeats. Record duplicate handling is separate.
		if number != 1 && number <= 7 {
			mask := uint8(1 << number)
			if seen&mask != 0 {
				return ErrMigration
			}
			seen |= mask
		}
		if err := consume(number, kind, field); err != nil {
			return err
		}
		data = data[length:]
	}
	return nil
}

func parseMigrationRecord(data []byte) (MigrationRecord, error) {
	var record MigrationRecord
	var secret []byte
	var algorithm, digits, kind uint64
	seenSecret := false
	err := walkMigration(data, func(number protowire.Number, wire protowire.Type, field []byte) error {
		if number >= 1 && number <= 3 {
			if wire != protowire.BytesType {
				return ErrMigration
			}
			if number == 1 {
				if seenSecret || len(field) == 0 || len(field) > 1024 {
					return ErrMigration
				}
				seenSecret = true
				secret = field
			} else {
				if len(field) > 1024 || !utf8.Valid(field) || strings.IndexFunc(string(field), unicode.IsControl) >= 0 {
					return ErrMigration
				}
				if number == 2 {
					record.Name = string(field)
				} else {
					record.Issuer = string(field)
				}
			}
		} else if number >= 4 && number <= 7 {
			if wire != protowire.VarintType {
				return ErrMigration
			}
			n, _ := protowire.ConsumeVarint(field)
			switch number {
			case 4:
				algorithm = n
			case 5:
				digits = n
			case 6:
				kind = n
			}
		}
		return nil
	})
	if err != nil || !seenSecret {
		return MigrationRecord{}, ErrMigration
	}
	if kind == 1 {
		record.Unsupported = "HOTP is not supported"
		return record, nil
	}
	if kind > 2 {
		record.Unsupported = "Unsupported OTP type"
		return record, nil
	}
	algorithms := []string{"SHA1", "SHA1", "SHA256", "SHA512"}
	if algorithm >= uint64(len(algorithms)) {
		record.Unsupported = "Unsupported hash algorithm"
		return record, nil
	}
	count := 6
	if digits == 2 {
		count = 8
	} else if digits > 2 {
		record.Unsupported = "Unsupported digit count"
		return record, nil
	}
	name := record.Name
	if issuer, account, ok := strings.Cut(name, ":"); ok {
		if record.Issuer == "" {
			record.Issuer = issuer
		}
		if record.Issuer == issuer {
			name = strings.TrimSpace(account)
		}
	}
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name+record.Issuer, ":") {
		record.Unsupported = "Account label needs manual correction"
		return record, nil
	}
	label := name
	if record.Issuer != "" {
		label = record.Issuer + ":" + name
	}
	q := url.Values{
		"secret":    {base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)},
		"algorithm": {algorithms[algorithm]},
		"digits":    {strconv.Itoa(count)},
		"period":    {"30"},
	}
	if record.Issuer != "" {
		q.Set("issuer", record.Issuer)
	}
	u := url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + label, RawQuery: q.Encode()}
	record.URI = u.String()
	if _, err := Parse(record.URI); err != nil {
		return MigrationRecord{}, ErrMigration
	}
	return record, nil
}

type MigrationBatch struct {
	pages         map[int]MigrationPage
	size, version int
	id            int32
	count         int
}

func (b *MigrationBatch) Progress() (loaded, total int) {
	return len(b.pages), b.size
}

func (b *MigrationBatch) HasPage(index int) bool {
	_, ok := b.pages[index]
	return ok
}

func (b *MigrationBatch) Add(page MigrationPage) error {
	if !validMigrationPage(page) {
		return ErrMigration
	}
	if b.pages != nil {
		if page.ID != b.id || page.Size != b.size || page.Version != b.version {
			return ErrMigrationBatch
		}
		if existing, ok := b.pages[page.Index]; ok {
			if existing.digest != page.digest {
				return ErrMigrationBatch
			}
			return nil
		}
	}
	if b.count+len(page.Records) > maxMigrationRecords {
		return ErrMigration
	}
	if b.pages == nil {
		b.pages = make(map[int]MigrationPage)
		b.id, b.size, b.version = page.ID, page.Size, page.Version
	}
	page.Records = append([]MigrationRecord(nil), page.Records...)
	b.pages[page.Index] = page
	b.count += len(page.Records)
	return nil
}

// AddPages commits an image's pages together so a rejected image cannot partly
// contaminate the batch being collected.
func (b *MigrationBatch) AddPages(pages []MigrationPage) error {
	staged := *b
	staged.pages = make(map[int]MigrationPage, len(b.pages)+len(pages))
	for index, page := range b.pages {
		staged.pages[index] = page
	}
	if b.pages == nil {
		staged.pages = nil
	}
	for _, page := range pages {
		if err := staged.Add(page); err != nil {
			return err
		}
	}
	*b = staged
	return nil
}

func validMigrationPage(page MigrationPage) bool {
	return (page.Version == 1 || page.Version == 2) &&
		page.Size >= 1 && page.Size <= maxMigrationPages &&
		page.Index >= 0 && page.Index < page.Size && len(page.Records) > 0
}

func (b *MigrationBatch) Missing() []int {
	var missing []int
	for i := 0; i < b.size; i++ {
		if _, ok := b.pages[i]; !ok {
			missing = append(missing, i)
		}
	}
	return missing
}

func (b *MigrationBatch) Records() ([]MigrationRecord, error) {
	if b.pages == nil || len(b.pages) != b.size {
		return nil, ErrMigrationBatch
	}
	result := make([]MigrationRecord, 0, b.count)
	for i := 0; i < b.size; i++ {
		result = append(result, b.pages[i].Records...)
	}
	return result, nil
}
