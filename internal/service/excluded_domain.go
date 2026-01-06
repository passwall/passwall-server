package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
)

type excludedDomainService struct {
	repo   repository.ExcludedDomainRepository
	logger Logger
}

// NewExcludedDomainService creates a new excluded domain service
func NewExcludedDomainService(
	repo repository.ExcludedDomainRepository,
	logger Logger,
) ExcludedDomainService {
	return &excludedDomainService{
		repo:   repo,
		logger: logger,
	}
}

func (s *excludedDomainService) Create(ctx context.Context, userID uint, req *domain.CreateExcludedDomainRequest) (*domain.ExcludedDomain, error) {
	// Normalize domain (extract hostname from URL if needed)
	normalizedDomain, err := normalizeDomain(req.Domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}

	// Check if already excluded
	existing, err := s.repo.GetByUserIDAndDomain(ctx, userID, normalizedDomain)
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("domain already excluded")
	}

	// Create excluded domain
	excludedDomain := &domain.ExcludedDomain{
		UUID:   uuid.NewV4(),
		UserID: userID,
		Domain: normalizedDomain,
	}

	if err := s.repo.Create(ctx, excludedDomain); err != nil {
		s.logger.Error("failed to create excluded domain", "user_id", userID, "domain", normalizedDomain, "error", err)
		return nil, err
	}

	s.logger.Info("excluded domain created", "user_id", userID, "domain", normalizedDomain)
	return excludedDomain, nil
}

func (s *excludedDomainService) GetByUserID(ctx context.Context, userID uint) ([]*domain.ExcludedDomain, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *excludedDomainService) Delete(ctx context.Context, id uint, userID uint) error {
	if err := s.repo.Delete(ctx, id, userID); err != nil {
		if err == repository.ErrNotFound {
			return fmt.Errorf("excluded domain not found")
		}
		s.logger.Error("failed to delete excluded domain", "id", id, "user_id", userID, "error", err)
		return err
	}

	s.logger.Info("excluded domain deleted", "id", id, "user_id", userID)
	return nil
}

func (s *excludedDomainService) DeleteByDomain(ctx context.Context, userID uint, domain string) error {
	normalizedDomain, err := normalizeDomain(domain)
	if err != nil {
		return fmt.Errorf("invalid domain: %w", err)
	}

	if err := s.repo.DeleteByDomain(ctx, userID, normalizedDomain); err != nil {
		if err == repository.ErrNotFound {
			return fmt.Errorf("excluded domain not found")
		}
		s.logger.Error("failed to delete excluded domain by domain", "user_id", userID, "domain", normalizedDomain, "error", err)
		return err
	}

	s.logger.Info("excluded domain deleted by domain", "user_id", userID, "domain", normalizedDomain)
	return nil
}

func (s *excludedDomainService) IsExcluded(ctx context.Context, userID uint, domain string) (bool, error) {
	normalizedDomain, err := normalizeDomain(domain)
	if err != nil {
		return false, nil // Invalid domain = not excluded
	}

	_, err = s.repo.GetByUserIDAndDomain(ctx, userID, normalizedDomain)
	if err == repository.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// normalizeDomain extracts the hostname from a URL or domain string
// Examples:
//   - "https://example.com/path" -> "example.com"
//   - "example.com" -> "example.com"
//   - "www.example.com" -> "example.com" (removes www)
func normalizeDomain(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("domain cannot be empty")
	}

	input = strings.TrimSpace(input)

	// Try to parse as URL first
	if strings.Contains(input, "://") || strings.HasPrefix(input, "//") {
		parsedURL, err := url.Parse(input)
		if err == nil && parsedURL.Host != "" {
			input = parsedURL.Host
		}
	}

	// Remove port if present
	if idx := strings.Index(input, ":"); idx > 0 {
		input = input[:idx]
	}

	// Remove www prefix
	input = strings.TrimPrefix(input, "www.")

	// Convert to lowercase
	input = strings.ToLower(input)

	// Basic validation
	if !strings.Contains(input, ".") {
		return "", fmt.Errorf("invalid domain format")
	}

	return input, nil
}
