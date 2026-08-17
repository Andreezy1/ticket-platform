package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------
// helpers — ключи генерируются в тесте, ничего не кладём в testdata/*.pem
// (в .gitignore *.pem и **/keys/, закоммитить их всё равно не выйдет).
// ---------------------------------------------------------------------

func generateEdKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	return pub, priv
}

// writePKCS8PEM маршалит любой поддерживаемый x509.MarshalPKCS8PrivateKey
// ключ, оборачивает в PEM и пишет во временный файл. Возвращает путь.
func writePKCS8PEM(t *testing.T, dir, filename string, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal PKCS8 key: %v", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	defer f.Close()
	if err := pem.Encode(f, block); err != nil {
		t.Fatalf("failed to pem-encode key: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------
// Этап A. computeKID — детерминизм и формат
// ---------------------------------------------------------------------

// base64url без паддинга: только [A-Za-z0-9_-], без '=', '+', '/'.
var base64URLNoPad = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func TestComputeKID_Deterministic(t *testing.T) {
	pub, _ := generateEdKey(t)

	kid1 := computeKID(pub)
	kid2 := computeKID(pub)

	if kid1 != kid2 {
		t.Errorf("computeKID not deterministic: %q != %q for the same key", kid1, kid2)
	}
}

func TestComputeKID_DifferentKeysDifferentKID(t *testing.T) {
	pub1, _ := generateEdKey(t)
	pub2, _ := generateEdKey(t)

	kid1 := computeKID(pub1)
	kid2 := computeKID(pub2)

	if kid1 == kid2 {
		t.Errorf("two different keys produced the same kid: %q", kid1)
	}
}

// Криптографические функции формата/совместимости тестируем известным
// вектором, а не случайным ключом — иначе тест "моргает": '+' и '/'
// встречаются в std-alphabet base64 не при каждом случайном ключе, и
// RawStdEncoding-мутация ловится только в части прогонов.
//
// Вектор: 32 нулевых байта в роли publicKey.
//
//	sha256(32×0x00)[:16] hex = 66687aadf862bd776c8fc18b8e9f8e20
//	RawURLEncoding  -> "Zmh6rfhivXdsj8GLjp-OIA"  (ожидаемый, правильный kid)
//	RawStdEncoding  -> "Zmh6rfhivXdsj8GLjp+OIA"  (содержит '+' — если бы
//	                    computeKID перепутали на std-алфавит, тест бы это
//	                    поймал детерминированно, на 100% прогонов).
func TestComputeKID_FixedVector_UsesURLSafeAlphabet(t *testing.T) {
	fixedPub := make(ed25519.PublicKey, ed25519.PublicKeySize) // 32 нулевых байта

	const wantKID = "Zmh6rfhivXdsj8GLjp-OIA"
	got := computeKID(fixedPub)

	if got != wantKID {
		t.Errorf("computeKID(fixed vector) = %q, want %q (RawURLEncoding vs RawStdEncoding mismatch?)", got, wantKID)
	}
}

func TestComputeKID_Format(t *testing.T) {
	pub, _ := generateEdKey(t)
	kid := computeKID(pub)

	// Регулярка сама по себе уже исключает '=', '+', '/' (класс символов —
	// ровно [A-Za-z0-9_-]), отдельная проверка ContainsAny избыточна и
	// оставлена только как явная документация намерения; реальную защиту
	// от подмены алфавита даёт TestComputeKID_FixedVector_UsesURLSafeAlphabet
	// выше, не эта проверка на случайном ключе.
	if !base64URLNoPad.MatchString(kid) {
		t.Errorf("kid %q is not valid unpadded base64url", kid)
	}
}

func TestComputeKID_Length(t *testing.T) {
	pub, _ := generateEdKey(t)
	kid := computeKID(pub)

	// Ловит мутацию sum[:16] -> sum[:] (весь sha256, 32 байта -> 43 символа
	// вместо 16 байт -> 22 символа).
	const wantLen = 22
	if len(kid) != wantLen {
		t.Errorf("len(kid) = %d, want %d (kid=%q)", len(kid), wantLen, kid)
	}

	// Дополнительно проверяем через декодирование: должно получиться
	// ровно 16 байт.
	decoded, err := base64.RawURLEncoding.DecodeString(kid)
	if err != nil {
		t.Fatalf("kid does not decode as base64url: %v", err)
	}
	if len(decoded) != 16 {
		t.Errorf("decoded kid length = %d bytes, want 16", len(decoded))
	}
}

// ---------------------------------------------------------------------
// Этап B. HashRefreshToken
// ---------------------------------------------------------------------

func newTestTokenService(t *testing.T) *TokenService {
	t.Helper()
	pub, priv := generateEdKey(t)
	return NewTokenService(priv, pub, "test-issuer", 15*time.Minute, 30*24*time.Hour)
}

func TestHashRefreshToken_Deterministic(t *testing.T) {
	svc := newTestTokenService(t)

	h1 := svc.HashRefreshToken("some-plain-value")
	h2 := svc.HashRefreshToken("some-plain-value")

	if h1 != h2 {
		t.Errorf("HashRefreshToken not deterministic: %q != %q", h1, h2)
	}
}

func TestHashRefreshToken_DifferentInputsDifferentHashes(t *testing.T) {
	svc := newTestTokenService(t)

	h1 := svc.HashRefreshToken("value-one")
	h2 := svc.HashRefreshToken("value-two")

	if h1 == h2 {
		t.Errorf("different inputs produced the same hash: %q", h1)
	}
}

func TestHashRefreshToken_Length(t *testing.T) {
	svc := newTestTokenService(t)
	h := svc.HashRefreshToken("anything")

	const wantLen = 64 // sha256 -> 32 bytes -> 64 hex chars
	if len(h) != wantLen {
		t.Errorf("len(hash) = %d, want %d (hash=%q)", len(h), wantLen, h)
	}
}

// Регрессия того самого бага в Logout: в репозиторий уезжал сам plaintext
// вместо хеша.
func TestHashRefreshToken_HashIsNotThePlaintext(t *testing.T) {
	svc := newTestTokenService(t)
	plain := "super-secret-refresh-token-plaintext"

	h := svc.HashRefreshToken(plain)

	if h == plain {
		t.Fatal("HashRefreshToken returned the plaintext unchanged")
	}
	if strings.Contains(h, plain) {
		t.Errorf("hash %q leaks the plaintext %q", h, plain)
	}
}

// ---------------------------------------------------------------------
// Этап C. GenerateRefreshToken — связь plain и hash
// ---------------------------------------------------------------------

func TestGenerateRefreshToken_HashMatchesHashRefreshTokenOfPlain(t *testing.T) {
	svc := newTestTokenService(t)

	plain, hash, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ключевая проверка: то, что уходит клиенту (plain), должно совпадать
	// по хешу с тем, что кладётся в БД (hash). Разъедутся — refresh
	// перестанет работать вообще, потому что Refresh будет искать по
	// HashRefreshToken(присланный plain) и не найдёт запись.
	want := svc.HashRefreshToken(plain)
	if hash != want {
		t.Errorf("hash = %q, want HashRefreshToken(plain) = %q", hash, want)
	}
}

func TestGenerateRefreshToken_PlainNotEqualHash(t *testing.T) {
	svc := newTestTokenService(t)

	plain, hash, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain == hash {
		t.Fatal("plain and hash are equal, hashing did not happen")
	}
}

func TestGenerateRefreshToken_TwoCallsProduceDifferentValues(t *testing.T) {
	svc := newTestTokenService(t)

	plain1, hash1, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	plain2, hash2, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	// Ловит "константу вместо генерации": если buf не читается из
	// crypto/rand (или читается, но игнорируется), фейки будут получать
	// одну и ту же пустую/фиксированную строку и проверки не смогут упасть.
	if plain1 == plain2 {
		t.Errorf("two calls produced the same plain token: %q", plain1)
	}
	if hash1 == hash2 {
		t.Errorf("two calls produced the same hash: %q", hash1)
	}
	if plain1 == "" || plain2 == "" {
		t.Fatal("plain token must not be empty")
	}
}

func TestGenerateRefreshToken_PlainLength(t *testing.T) {
	svc := newTestTokenService(t)

	plain, _, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const wantLen = 43 // 32 bytes -> RawURLEncoding -> 43 chars (no padding)
	if len(plain) != wantLen {
		t.Errorf("len(plain) = %d, want %d (plain=%q)", len(plain), wantLen, plain)
	}
}

func TestGenerateRefreshToken_PlainIsValidBase64URL(t *testing.T) {
	svc := newTestTokenService(t)

	plain, _, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.ContainsAny(plain, "=+/") {
		t.Errorf("plain contains non-base64url characters (=, +, or /): %q", plain)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(plain)
	if err != nil {
		t.Fatalf("plain does not decode as base64url: %v", err)
	}
	if len(decoded) != refreshEntropyBytes {
		t.Errorf("decoded plain length = %d bytes, want %d (entropy source not fully used?)", len(decoded), refreshEntropyBytes)
	}
}

// ---------------------------------------------------------------------
// Этап D. IssueAccessToken — самый содержательный
// ---------------------------------------------------------------------

// mustIssueAndParse выпускает токен и парсит его тем же публичным ключом,
// с проверкой подписи и метода — если что-то из этого не так, тест,
// использующий хелпер, должен упасть сразу здесь, а не молча продолжить
// с невалидным токеном.
func mustIssueAndParse(t *testing.T, svc *TokenService, userID, role string) (string, *jwt.Token, *Claims) {
	t.Helper()

	tokenString, err := svc.IssueAccessToken(userID, role)
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (interface{}, error) {
		return svc.pub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil {
		t.Fatalf("failed to parse issued token: %v", err)
	}
	if !tok.Valid {
		t.Fatalf("issued token did not validate")
	}

	return tokenString, tok, claims
}

func TestIssueAccessToken_HeaderAlgIsEdDSA(t *testing.T) {
	svc := newTestTokenService(t)
	_, tok, _ := mustIssueAndParse(t, svc, "user-1", "admin")

	alg, _ := tok.Header["alg"].(string)
	// Ловит подмену алгоритма подписи.
	if alg != "EdDSA" {
		t.Errorf("header alg = %q, want %q", alg, "EdDSA")
	}
}

func TestIssueAccessToken_HeaderKidMatchesComputeKID(t *testing.T) {
	svc := newTestTokenService(t)
	_, tok, _ := mustIssueAndParse(t, svc, "user-1", "admin")

	kid, _ := tok.Header["kid"].(string)
	want := computeKID(svc.pub)

	// Ловит рассинхрон с JWKS: если kid в токене вычисляется иначе, чем
	// computeKID(pub) (например, кэшируется неправильно или пересчитывается
	// по другому ключу), клиент не найдёт подходящий ключ в /jwks.json.
	if kid != want {
		t.Errorf("header kid = %q, want computeKID(pub) = %q", kid, want)
	}
	if kid != svc.kid {
		t.Errorf("header kid = %q, want svc.kid (cached at construction) = %q", kid, svc.kid)
	}
}

func TestIssueAccessToken_Claims(t *testing.T) {
	svc := newTestTokenService(t) // issuer = "test-issuer", см. newTestTokenService
	_, _, claims := mustIssueAndParse(t, svc, "user-42", "admin")

	// Ловит перепутанные местами аргументы/поля claims.
	if claims.Subject != "user-42" {
		t.Errorf("sub = %q, want %q", claims.Subject, "user-42")
	}
	if claims.Role != "admin" {
		t.Errorf("role = %q, want %q", claims.Role, "admin")
	}
	if claims.Issuer != "test-issuer" {
		t.Errorf("iss = %q, want %q", claims.Issuer, "test-issuer")
	}
}

func TestIssueAccessToken_ExpMinusIatEqualsAccessTTL(t *testing.T) {
	// accessTTL и refreshTTL намеренно РАЗНЫЕ и оба непохожи на дефолт,
	// чтобы тест падал, если их перепутают местами внутри TokenService
	// (одна буква разницы в имени поля — а последствие: access-токен,
	// живущий как refresh, то есть 30 дней вместо 15 минут).
	const accessTTL = 15 * time.Minute
	const refreshTTL = 30 * 24 * time.Hour

	pub, priv := generateEdKey(t)
	svc := NewTokenService(priv, pub, "test-issuer", accessTTL, refreshTTL)

	_, _, claims := mustIssueAndParse(t, svc, "user-1", "admin")

	iat := claims.IssuedAt.Time
	exp := claims.ExpiresAt.Time
	got := exp.Sub(iat)

	if got != accessTTL {
		t.Errorf("exp - iat = %v, want %v (accessTokenTTL vs refreshTokenTTL swapped?)", got, accessTTL)
	}
}

func TestIssueAccessToken_JTI_NonEmptyAndUniquePerToken(t *testing.T) {
	svc := newTestTokenService(t)

	_, _, claims1 := mustIssueAndParse(t, svc, "user-1", "admin")
	_, _, claims2 := mustIssueAndParse(t, svc, "user-1", "admin")

	if claims1.ID == "" {
		t.Fatal("jti is empty")
	}
	// Ловит константу вместо генерации UUID.
	if claims1.ID == claims2.ID {
		t.Errorf("two independently issued tokens got the same jti: %q", claims1.ID)
	}
}

func TestIssueAccessToken_SignatureValidatesWithCorrectPublicKey(t *testing.T) {
	svc := newTestTokenService(t)
	tokenString, err := svc.IssueAccessToken("user-1", "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = jwt.ParseWithClaims(tokenString, &Claims{}, func(*jwt.Token) (interface{}, error) {
		return svc.pub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil {
		t.Errorf("token should verify with the correct public key, got error: %v", err)
	}
}

func TestIssueAccessToken_SignatureRejectedWithWrongPublicKey(t *testing.T) {
	svc := newTestTokenService(t)
	tokenString, err := svc.IssueAccessToken("user-1", "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wrongPub, _ := generateEdKey(t)

	// Проверка самой проверки: если подпись валидна ЛЮБЫМ ключом, значит
	// верификация не выполняет свою единственную работу.
	_, err = jwt.ParseWithClaims(tokenString, &Claims{}, func(*jwt.Token) (interface{}, error) {
		return wrongPub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err == nil {
		t.Fatal("token unexpectedly verified with a different public key — signature check is not actually checking anything")
	}
}

// ---------------------------------------------------------------------
// Этап E. JWKS — связь с токеном
// ---------------------------------------------------------------------

func TestJWKS_ExactlyOneKey(t *testing.T) {
	svc := newTestTokenService(t)
	keys := svc.JWKS()

	if len(keys) != 1 {
		t.Fatalf("len(JWKS()) = %d, want 1", len(keys))
	}
}

func TestJWKS_StaticFields(t *testing.T) {
	svc := newTestTokenService(t)
	k := svc.JWKS()[0]

	if k.Kty != "OKP" {
		t.Errorf("kty = %q, want %q", k.Kty, "OKP")
	}
	if k.Crv != "Ed25519" {
		t.Errorf("crv = %q, want %q", k.Crv, "Ed25519")
	}
	if k.Use != "sig" {
		t.Errorf("use = %q, want %q", k.Use, "sig")
	}
	if k.Alg != "EdDSA" {
		t.Errorf("alg = %q, want %q", k.Alg, "EdDSA")
	}
}

func TestJWKS_XDecodesTo32Bytes(t *testing.T) {
	svc := newTestTokenService(t)
	k := svc.JWKS()[0]

	decoded, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		t.Fatalf("x does not decode as base64url: %v", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Errorf("decoded x length = %d bytes, want %d", len(decoded), ed25519.PublicKeySize)
	}
}

func TestJWKS_XMatchesPublicKey(t *testing.T) {
	svc := newTestTokenService(t)
	k := svc.JWKS()[0]

	decoded, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		t.Fatalf("x does not decode as base64url: %v", err)
	}

	// Ловит "чужой ключ": x должен быть именно тем публичным ключом,
	// которым подписываются токены, а не каким-то другим.
	if !ed25519.PublicKey(decoded).Equal(svc.pub) {
		t.Error("JWKS x does not match the service's public key")
	}
}

func TestJWKS_KidMatchesIssuedTokenHeaderKid(t *testing.T) {
	svc := newTestTokenService(t)
	_, tok, _ := mustIssueAndParse(t, svc, "user-1", "admin")

	headerKid, _ := tok.Header["kid"].(string)
	jwksKid := svc.JWKS()[0].Kid

	// Главная проверка этапа: связь между JWKS и заголовком токена делается
	// явной, а не держится только на том, что оба места зовут computeKID.
	if headerKid != jwksKid {
		t.Errorf("token header kid = %q, JWKS kid = %q, must match", headerKid, jwksKid)
	}
}

func TestJWKS_NoPrivateKeyMaterialLeaked(t *testing.T) {
	svc := newTestTokenService(t)

	data, err := json.Marshal(svc.JWKS())
	if err != nil {
		t.Fatalf("failed to marshal JWKS: %v", err)
	}
	body := string(data)

	// Приватный ключ (в любой из его типовых текстовых форм) не должен
	// попасть в публичный ответ ни в каком виде.
	privRawURL := base64.RawURLEncoding.EncodeToString(svc.priv)
	privStd := base64.StdEncoding.EncodeToString(svc.priv)
	if strings.Contains(body, privRawURL) || strings.Contains(body, privStd) {
		t.Error("JWKS response leaks private key material")
	}
}

// ---------------------------------------------------------------------
// Этап F. LoadPrivateKey — файловые тесты. Ключи генерируются на лету и
// пишутся в t.TempDir(), НИЧЕГО не коммитится в testdata/*.pem (см. план).
// ---------------------------------------------------------------------

func TestLoadPrivateKey_Success(t *testing.T) {
	pub, priv := generateEdKey(t)
	dir := t.TempDir()
	path := writePKCS8PEM(t, dir, "key.pem", priv)

	loadedPriv, loadedPub, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !loadedPriv.Equal(priv) {
		t.Error("loaded private key does not match the generated one")
	}
	if !loadedPub.Equal(pub) {
		t.Error("loaded public key does not match the generated one")
	}
	// Публичный ключ обязан быть производным от приватного, а не просто
	// "каким-то" ed25519.PublicKey.
	if !loadedPub.Equal(loadedPriv.Public().(ed25519.PublicKey)) {
		t.Error("returned public key is not derived from the returned private key")
	}
}

func TestLoadPrivateKey_FileNotFound(t *testing.T) {
	priv, pub, err := LoadPrivateKey(filepath.Join(t.TempDir(), "does-not-exist.pem"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if priv != nil {
		t.Errorf("expected nil private key on error, got %v", priv)
	}
	if pub != nil {
		t.Errorf("expected nil public key on error, got %v", pub)
	}
}

func TestLoadPrivateKey_MalformedPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.pem")
	if err := os.WriteFile(path, []byte("this is not a pem file at all"), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	priv, pub, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected error for malformed PEM, got nil")
	}
	if priv != nil {
		t.Errorf("expected nil private key on error, got %v", priv)
	}
	if pub != nil {
		t.Errorf("expected nil public key on error, got %v", pub)
	}
}

func TestLoadPrivateKey_CorruptedDER(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.pem")

	// Валидный PEM-конверт, но внутри — не ASN.1/DER вовсе. Это ветка
	// x509.ParsePKCS8PrivateKey возвращает ошибку, а не тип-ассершн
	// на ed25519.PrivateKey (ту проверяет TestLoadPrivateKey_WrongKeyType).
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("definitely not valid DER bytes")}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	defer f.Close()
	if err := pem.Encode(f, block); err != nil {
		t.Fatalf("failed to pem-encode key: %v", err)
	}

	priv, pub, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected error for corrupted DER content, got nil")
	}
	if priv != nil {
		t.Errorf("expected nil private key on error, got %v", priv)
	}
	if pub != nil {
		t.Errorf("expected nil public key on error, got %v", pub)
	}
}

func TestLoadPrivateKey_WrongKeyType(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	dir := t.TempDir()
	path := writePKCS8PEM(t, dir, "rsa.pem", rsaKey)

	priv, pub, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected error when key is not ed25519, got nil")
	}
	if priv != nil {
		t.Errorf("expected nil private key on error, got %v", priv)
	}
	if pub != nil {
		t.Errorf("expected nil public key on error, got %v", pub)
	}
}
