// backend/internal/reset/report.go
package reset

// Report is the typed summary of a destructive admin operation. Every
// field is an absolute count. Consumers (HTTP handler, audit log) read
// it directly — there is no separate DTO layer. A zero-valued Report
// means "nothing was touched" and is valid (e.g. WipeInvites on an
// already-empty invites table).
type Report struct {
	RoomsDeleted         int64  `json:"rooms_deleted"`
	RoomPlayersDeleted   int64  `json:"room_players_deleted"`
	RoundsDeleted        int64  `json:"rounds_deleted"`
	SubmissionsDeleted   int64  `json:"submissions_deleted"`
	VotesDeleted         int64  `json:"votes_deleted"`
	PacksDeleted         int64  `json:"packs_deleted"`
	ItemsDeleted         int64  `json:"items_deleted"`
	VersionsDeleted      int64  `json:"versions_deleted"`
	InvitesDeleted       int64  `json:"invites_deleted"`
	SessionsDeleted      int64  `json:"sessions_deleted"`
	MagicTokensDeleted   int64  `json:"magic_tokens_deleted"`
	NotificationsDeleted int64  `json:"notifications_deleted"`
	UsersDeleted         int64  `json:"users_deleted"`
	S3ObjectsDeleted     int64  `json:"s3_objects_deleted"`
	S3Error              string `json:"s3_error,omitempty"`
	ExcludedSelf         bool   `json:"excluded_self,omitempty"`
}

// merge adds every count field from other into r. S3Error follows
// "first error wins" semantics — subsequent errors are ignored because
// the first failure is the root cause.
func (r *Report) merge(other Report) {
	r.RoomsDeleted += other.RoomsDeleted
	r.RoomPlayersDeleted += other.RoomPlayersDeleted
	r.RoundsDeleted += other.RoundsDeleted
	r.SubmissionsDeleted += other.SubmissionsDeleted
	r.VotesDeleted += other.VotesDeleted
	r.PacksDeleted += other.PacksDeleted
	r.ItemsDeleted += other.ItemsDeleted
	r.VersionsDeleted += other.VersionsDeleted
	r.InvitesDeleted += other.InvitesDeleted
	r.SessionsDeleted += other.SessionsDeleted
	r.MagicTokensDeleted += other.MagicTokensDeleted
	r.NotificationsDeleted += other.NotificationsDeleted
	r.UsersDeleted += other.UsersDeleted
	r.S3ObjectsDeleted += other.S3ObjectsDeleted
	if r.S3Error == "" && other.S3Error != "" {
		r.S3Error = other.S3Error
	}
	if other.ExcludedSelf {
		r.ExcludedSelf = true
	}
}
