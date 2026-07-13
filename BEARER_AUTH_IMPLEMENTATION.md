# Bearer Token Authentication Implementation

## Overview

Implemented provider-scoped Bearer token authentication for API endpoints. This prevents cross-provider token leakage by requiring clients to explicitly specify which provider their token belongs to.

## Changes Made

### 1. Middleware (`internal/frontend/auth/middleware.go`)

#### New Constants
- `headerAuthProvider = "X-Auth-Provider"` - Header name for provider specification

#### New Functions
- `tryBearerTokenAuth()` - Extracts and validates Bearer token with provider header
- `validateBearerToken()` - Validates token against a specific provider only (no fallback)

#### Modified Functions
- `authCheckMiddleware()` - Now checks for Bearer token on API paths and extracts provider header
- `handleBearerTokenAuth()` - Accepts `providerName` parameter
- `extractBearerToken()` - Case-insensitive Bearer scheme matching (RFC 6750 compliant)

#### Key Features
- **Provider-scoped validation**: Tokens are only validated against the specified provider
- **Provider-scoped caching**: Cache keys include provider name (`provider:hash`)
- **Explicit provider requirement**: Missing `X-Auth-Provider` header returns 401
- **No cross-provider leakage**: Tokens never sent to providers other than the specified one

### 2. Tests (`internal/frontend/auth/middleware_test.go`)

#### Updated Tests
- All Bearer token tests now include `X-Auth-Provider` header
- Added test for missing provider header (returns 401)
- Added test for unknown provider (returns 401)
- Added test for invalid token with valid provider (returns 401)
- Added test for valid token with valid provider (returns 200)

### 3. Provider Implementations

#### GitHub Provider (`internal/frontend/auth/github/provider.go`)
- Added `ValidateToken()` method using go-github SDK
- Fixed TLS configuration for GHES (GitHub Enterprise Server) with self-signed certs
- Passes `p.httpClient` to SDK to preserve TLS settings

#### Gitea Provider (`internal/frontend/auth/gitea/provider.go`)
- Added `ValidateToken()` method using gitea SDK
- Uses `gitea.NewClient()` with token authentication

### 4. Session Management (`internal/frontend/auth/session.go`)

#### New Functions
- `SetAPISessionData()` - Stores session data in request context for API auth
- `GetAPISessionData()` - Retrieves API session data from context

#### Modified Functions
- `GetSessionData()` - Now checks API session context first, then falls back to cookie session

### 5. Auth Provider Interface (`internal/frontend/auth/provider.go`)

#### New Interface Method
```go
ValidateToken(ctx context.Context, token string) (*AuthenticatedUser, error)
```

All auth providers must implement this method.

## API Usage

### Request Format
```bash
curl -H "Authorization: Bearer <token>" \
     -H "X-Auth-Provider: <provider-name>" \
     https://operator.example.com/api/v1/renovators
```

### Example
```bash
# Validate against GitHub provider
curl -H "Authorization: Bearer ghp_xxxxxxxxxxxx" \
     -H "X-Auth-Provider: github-prod" \
     https://operator.example.com/api/v1/renovators

# Validate against Gitea provider
curl -H "Authorization: Bearer gitea_token_xxxx" \
     -H "X-Auth-Provider: gitea-prod" \
     https://operator.example.com/api/v1/renovators
```

### Error Responses

#### Missing Provider Header
```json
{
  "error": "missing X-Auth-Provider header"
}
```

#### Invalid Token
```json
{
  "error": "invalid token"
}
```

#### Unknown Provider
```json
{
  "error": "invalid provider"
}
```

## Security Benefits

1. **No Cross-Provider Token Leakage**: Tokens are only sent to the specified provider
2. **Explicit Provider Selection**: Clients must declare which provider their token belongs to
3. **Provider-Scoped Caching**: Prevents cache poisoning across providers
4. **RFC 6750 Compliant**: Case-insensitive Bearer scheme matching

## Implementation Notes

### Cache Key Format
```
<provider-name>:<sha256-hash-of-token>
```

Example: `github-prod:a1b2c3d4e5f6...`

### Provider Name
The provider name must match the name configured in the AuthProvider CRD, not the provider type. For example:
- ✅ `github-prod` (provider name)
- ❌ `github` (provider type)

### Token Validation Flow
1. Extract Bearer token from `Authorization` header
2. Extract provider name from `X-Auth-Provider` header
3. Check cache for `provider:hash(token)`
4. If cache miss, validate token against specified provider only
5. Cache successful validation for 5 minutes
6. Return session data or error

## Testing

All tests pass:
- Unit tests: ✅ 30/30 passed
- Linters: ✅ 0 issues
- Coverage: 82.5% for auth package

## Future Enhancements

Potential improvements for future iterations:
1. **Rate Limiting**: Add per-IP rate limiting on Bearer token validation
2. **Negative Caching**: Cache failed validations to prevent repeated API calls
3. **Token Format Inference**: Auto-detect provider from token format (e.g., `ghp_` → GitHub)
4. **Device Authorization Grant**: CLI-friendly OAuth flow for token generation
