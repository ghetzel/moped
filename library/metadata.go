package library

import "time"

type Metadata struct {
	Title    string                 `json:"title,omitempty"`
	Artist   string                 `json:"artist,omitempty"`
	Album    string                 `json:"album,omitempty"`
	Genre    string                 `json:"genre,omitempty"`
	Year     int                    `json:"year,omitempty"`
	Disc     int                    `json:"disc,omitempty"`
	Track    int                    `json:"track,omitempty"`
	Duration time.Duration          `json:"duration,omitempty"`
	Extra    map[string]interface{} `json:"extra,omitempty"`
}
