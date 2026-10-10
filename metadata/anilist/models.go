package anilist

import "github.com/luevano/libmangal/metadata"

type byIDData struct {
	Media *Manga `json:"media"`
}

type mangasData struct {
	Page struct {
		Media medias `json:"media"`
	} `json:"page"`
}

type medias []*Manga

func (m medias) GetAsMetas() []metadata.Metadata {
	mangas := make([]metadata.Metadata, len(m))
	for i, n := range m {
		mangas[i] = n
	}
	return mangas
}

type listEntryData struct {
	Media *struct {
		MediaListEntry *struct {
			Progress int    `json:"progress"`
			Status   string `json:"status"`
		} `json:"mediaListEntry"`
	} `json:"media"`
}

type setProgressData struct {
	SaveMediaListEntry struct {
		ID int `json:"id"`
	} `json:"SaveMediaListEntry"`
}

type userData struct {
	Viewer *User `json:"viewer"`
}
