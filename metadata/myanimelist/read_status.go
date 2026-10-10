package myanimelist

// TODO: change field types from string to date/date-time
//
// ReadStatus represents the MyAnimeList manga list read status.
type ReadStatus struct {
	Status          string   `json:"status,omitempty" jsonschema:"enum=reading,enum=completed,enum=on_hold,enum=dropped,enum=plan_to_read"`
	Score           int      `json:"score,omitempty" jsonschema:"description=0-10"`
	NumVolumesRead  int      `json:"num_volumes_read,omitempty" jsonschema:"description=0 or the number of read volumes."`
	NumChaptersRead int      `json:"num_chapters_read,omitempty" jsonschema:"description=0 or the number of read chapters."`
	IsRereading     bool     `json:"is_rereading,omitempty" jsonschema:"description=If authorized user reads an manga again after completion, this field value is true.\nIn this case, MyAnimeList treats the manga as 'reading' in the user's manga list."`
	StartDate       string   `json:"start_date,omitempty"`
	FinishDate      string   `json:"finish_date,omitempty"`
	Priority        int      `json:"priority,omitempty"`
	NumTimesReread  int      `json:"num_times_reread,omitempty"`
	RereadValue     int      `json:"reread_value,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Comments        string   `json:"comments,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
}
