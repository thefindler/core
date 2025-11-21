// auth/middleware.go
package auth

import (
	"strings"
	"net/http"

	"github.com/thefindler/core/pg/model"
	"github.com/gofiber/fiber/v2"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const AuthContextKey ContextKey = "authContext"

// MiddlewareService provides the authentication and authorization middleware.
type MiddlewareService struct {
	tokenService *TokenService
	db           model.DB // Direct DB access for org-specific secrets
}

// NewMiddlewareService creates a new middleware service.
func NewMiddlewareService(ts *TokenService, db model.DB) *MiddlewareService {
	return &MiddlewareService{
		tokenService: ts,
		db:           db,
	}
}

// Authenticate is the core middleware logic. It validates the token,
// checks the database, and injects the AuthContext into the request context.
func (s *MiddlewareService) Authenticate(c *fiber.Ctx) (*AuthContext, error) {
	var tokenString string
	
	// First, try to get token from Authorization header
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return nil, ErrInvalidToken
		}
		tokenString = parts[1]
	} else {
		// Fallback: Check for token in query parameters (needed for WebSocket connections)
		tokenString = c.Query("token")
		if tokenString == "" {
			return nil, ErrMissingToken
		}
	}

	// 1. Validate the token with the global secret
	claims, err := s.tokenService.ValidateToken(tokenString)
	if err != nil {
		return nil, err // ValidateToken returns ErrInvalidToken
	}

	if claims.TokenType != AccessToken {
		return nil, NewAuthError("INVALID_TOKEN_TYPE", "Access token required", 401)
	}

	// 2. Perform DB validation to ensure user/service is still active
	ctx := c.Context() // Use the Fiber context
	if claims.UserType == "service" {
		if err := s.db.ValidateServiceActive(ctx, claims.UserID); err != nil {
			return nil, ErrServiceInactive
		}
	} else {
		orgIDStr := ""
		if claims.OrganisationID != nil {
			orgIDStr = *claims.OrganisationID
		}
		if err := s.db.ValidateUserActive(ctx, claims.UserID, orgIDStr); err != nil {
			return nil, ErrUserInactive
		}
	}

	// 5. Create and return AuthContext
	authCtx := &AuthContext{
		UserID:         claims.UserID,
		OrganisationID: claims.OrganisationID,
		Username:       claims.Username,
		Role:           claims.Role,
		UserType:       claims.UserType,
		Permissions:    claims.Permissions,
	}

	return authCtx, nil
}

// Middleware is a factory for creating a new Fiber middleware that enforces
// a specific authorization requirement.
func (s *MiddlewareService) Middleware(requirement *AuthRequirement) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Authentication
		authCtx, err := s.Authenticate(c)
		if err != nil {
			return handleAuthError(c, err)
		}

		// Store AuthContext in Fiber's locals
		c.Locals(string(AuthContextKey), authCtx)

		// 2. Authorization
		if requirement != nil {
			// Role check
			if len(requirement.RequiredRoles) > 0 {
				if err := AuthorizeRole(c, requirement.RequiredRoles); err != nil {
					return handleAuthError(c, err)
				}
			}

			// Permission check
			if len(requirement.RequiredPermissions) > 0 {
				if err := AuthorizePermission(c, requirement.RequiredPermissions); err != nil {
					return handleAuthError(c, err)
				}
			}
		}

		// If all checks pass, proceed.
		return c.Next()
	}
}

// handleAuthError is a helper to write a structured auth error to the Fiber response.
func handleAuthError(c *fiber.Ctx, err error) error {
	if authErr, ok := err.(*AuthError); ok {
		return c.Status(authErr.Code).JSON(fiber.Map{"error": authErr.Message})
	}
	return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
}

// --- Authorization Helpers ---

// AuthorizeRole checks if the user in the context has one of the required roles.
func AuthorizeRole(c *fiber.Ctx, allowedRoles []string) error {
	authCtx, ok := c.Locals(string(AuthContextKey)).(*AuthContext)
	if !ok {
		return ErrMissingToken
	}

	for _, role := range allowedRoles {
		if authCtx.Role == role {
			return nil
		}
	}

	return ErrInsufficientRole
}

// AuthorizePermission checks if a service user has all required permissions.
func AuthorizePermission(c *fiber.Ctx, requiredPerms []string) error {
    authCtx, ok := c.Locals(string(AuthContextKey)).(*AuthContext)
	if !ok || authCtx.UserType != "service" {
		return ErrInsufficientPerms
	}

    // Create a map of the user's permissions for efficient lookup
    userPerms := make(map[string]struct{})
    for _, p := range authCtx.Permissions {
        userPerms[p] = struct{}{}
    }

    // Check if all required permissions are present
    for _, reqP := range requiredPerms {
        if _, ok := userPerms[reqP]; !ok {
            return ErrInsufficientPerms
        }
    }

	return nil
}