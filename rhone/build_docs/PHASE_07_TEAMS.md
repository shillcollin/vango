# Phase 7: Teams & Organizations

> **Multi-user collaboration with role-based access control**

**Status**: Not Started

---

## Overview

Phase 7 implements team management, allowing multiple users to collaborate on apps. Teams have owners, admins, and members with different permission levels.

### Goals

1. **Team creation**: Users can create teams/organizations
2. **Member invitations**: Invite users via email
3. **Role-based permissions**: Owner, admin, member roles
4. **Team switching**: Users can belong to multiple teams
5. **App ownership**: Apps belong to teams, not users

### Non-Goals

1. SSO/SAML (enterprise feature)
2. Fine-grained per-app permissions
3. Audit logging (Phase 12)

---

## Role Permissions

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     ROLE PERMISSIONS MATRIX                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Permission               │ Owner │ Admin │ Member │                    │
│  ─────────────────────────┼───────┼───────┼────────┤                    │
│  View apps                │   ✓   │   ✓   │   ✓    │                    │
│  Create apps              │   ✓   │   ✓   │   ✓    │                    │
│  Deploy apps              │   ✓   │   ✓   │   ✓    │                    │
│  View logs                │   ✓   │   ✓   │   ✓    │                    │
│  Manage env vars          │   ✓   │   ✓   │   ✗    │                    │
│  Delete apps              │   ✓   │   ✓   │   ✗    │                    │
│  Invite members           │   ✓   │   ✓   │   ✗    │                    │
│  Remove members           │   ✓   │   ✓   │   ✗    │                    │
│  Change member roles      │   ✓   │   ✗   │   ✗    │                    │
│  Manage billing           │   ✓   │   ✗   │   ✗    │                    │
│  Delete team              │   ✓   │   ✗   │   ✗    │                    │
│  Transfer ownership       │   ✓   │   ✗   │   ✗    │                    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema

```sql
-- Already created in Phase 1, but here for reference:
-- teams, team_members tables

-- Add invitation table
CREATE TABLE team_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    invited_by UUID REFERENCES users(id),
    token VARCHAR(64) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_team_invitations_email ON team_invitations(email);
CREATE INDEX idx_team_invitations_token ON team_invitations(token);
```

---

## Core Implementation

```go
// internal/domain/team.go
package domain

type Role string

const (
    RoleOwner  Role = "owner"
    RoleAdmin  Role = "admin"
    RoleMember Role = "member"
)

func (r Role) CanManageEnvVars() bool {
    return r == RoleOwner || r == RoleAdmin
}

func (r Role) CanDeleteApps() bool {
    return r == RoleOwner || r == RoleAdmin
}

func (r Role) CanInviteMembers() bool {
    return r == RoleOwner || r == RoleAdmin
}

func (r Role) CanManageBilling() bool {
    return r == RoleOwner
}

func (r Role) CanDeleteTeam() bool {
    return r == RoleOwner
}

// TeamService handles team operations
type TeamService struct {
    queries *queries.Queries
    mailer  *mail.Mailer
    logger  *slog.Logger
}

func (s *TeamService) CreateTeam(ctx context.Context, userID uuid.UUID, name, slug string) (*queries.Team, error) {
    // Create team
    team, err := s.queries.CreateTeam(ctx, queries.CreateTeamParams{
        Name: name,
        Slug: slug,
        Plan: "free",
    })
    if err != nil {
        return nil, err
    }

    // Add creator as owner
    _, err = s.queries.AddTeamMember(ctx, queries.AddTeamMemberParams{
        TeamID: team.ID,
        UserID: userID,
        Role:   string(RoleOwner),
    })
    if err != nil {
        return nil, err
    }

    return &team, nil
}

func (s *TeamService) InviteMember(ctx context.Context, teamID uuid.UUID, email string, role Role, invitedBy uuid.UUID) error {
    // Generate invitation token
    token := generateToken(32)

    // Create invitation
    _, err := s.queries.CreateInvitation(ctx, queries.CreateInvitationParams{
        TeamID:    teamID,
        Email:     email,
        Role:      string(role),
        InvitedBy: &invitedBy,
        Token:     token,
        ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
    })
    if err != nil {
        return err
    }

    // Send invitation email
    team, _ := s.queries.GetTeam(ctx, teamID)
    return s.mailer.SendInvitation(email, team.Name, token)
}

func (s *TeamService) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) error {
    // Find invitation
    invitation, err := s.queries.GetInvitationByToken(ctx, token)
    if err != nil {
        return fmt.Errorf("invitation not found")
    }

    if time.Now().After(invitation.ExpiresAt) {
        return fmt.Errorf("invitation expired")
    }

    if invitation.AcceptedAt != nil {
        return fmt.Errorf("invitation already used")
    }

    // Add user to team
    _, err = s.queries.AddTeamMember(ctx, queries.AddTeamMemberParams{
        TeamID: invitation.TeamID,
        UserID: userID,
        Role:   invitation.Role,
    })
    if err != nil {
        return err
    }

    // Mark invitation as accepted
    return s.queries.AcceptInvitation(ctx, invitation.ID)
}
```

---

## Middleware

```go
// internal/middleware/team.go
package middleware

func RequireTeamRole(minRole domain.Role) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            session := GetSession(r.Context())
            if session == nil {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            // Get user's role in current team
            membership := GetTeamMembership(r.Context())
            if membership == nil {
                http.Error(w, "Not a team member", http.StatusForbidden)
                return
            }

            userRole := domain.Role(membership.Role)

            // Check if user's role is sufficient
            if !roleAtLeast(userRole, minRole) {
                http.Error(w, "Insufficient permissions", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func roleAtLeast(userRole, required domain.Role) bool {
    roleOrder := map[domain.Role]int{
        domain.RoleMember: 1,
        domain.RoleAdmin:  2,
        domain.RoleOwner:  3,
    }
    return roleOrder[userRole] >= roleOrder[required]
}
```

---

## Exit Criteria

Phase 7 is complete when:

1. [ ] Users can create new teams
2. [ ] Team slugs are unique
3. [ ] Members can be invited via email
4. [ ] Invitations expire after 7 days
5. [ ] Users can accept invitations
6. [ ] Role permissions enforced correctly
7. [ ] Users can switch between teams
8. [ ] Team settings page works
9. [ ] Members can be removed
10. [ ] Ownership can be transferred

---

## Dependencies

- **Requires**: Phase 1 (authentication)
- **Required by**: Phase 8 (billing is per-team)

---

*Phase 7 Specification - Version 1.0*
