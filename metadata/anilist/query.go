package anilist

// queryCommon common manga query used for getting manga by id or searching it by name
const queryCommon = `
id
idMal
title {
	romaji
	english
	native
}
description(asHtml: false)
averageScore
tags {
	name
	description
	rank
}
genres
coverImage {
	extraLarge
	large
	medium
	color
}
bannerImage
characters (page: 1, perPage: 10, role: MAIN) {
	nodes {
		id
		name {
			full
			native
		}
	}
}
startDate {
	year
	month
	day
}
endDate {
	year
	month
	day
}
staff {
	edges {
	  role
	  node {
		name {
		  full
		}
	  }
	}
}
status
synonyms
siteUrl
chapters
countryOfOrigin
externalLinks {
	url
}
`

const querySearchByName = `
query ($query: String) {
	Page (page: 1, perPage: 30) {
		media (search: $query, type: MANGA) {
			` + queryCommon + `
		}
	}
}`

const querySearchByID = `
query ($id: Int) {
	Media (id: $id, type: MANGA) {
		` + queryCommon + `
	}
}`

// viewer is the currently authenticated user
const queryViewer = `
query {
	Viewer {
		id
		name
		about(asHtml: false)
		avatar {
			large
			medium
		}
		bannerImage
		options {
			titleLanguage
			displayAdultContent
			profileColor
			timezone
		}
		siteUrl
		createdAt
		updatedAt
		previousNames {
			name
			createdAt
			updatedAt
		}
	}
}`

// queryListEntry is the authenticated user's list entry of a manga, null if not in the list
const queryListEntry = `
query ($id: Int) {
	Media (id: $id, type: MANGA) {
		mediaListEntry {
			progress
			status
		}
	}
}`

// mutationSaveProgress keeps the entry's status
const mutationSaveProgress = `
mutation ($id: Int, $progress: Int) {
	SaveMediaListEntry (mediaId: $id, progress: $progress) {
		id
	}
}`

// mutationAddProgress adds the manga to the list as being read
const mutationAddProgress = `
mutation ($id: Int, $progress: Int) {
	SaveMediaListEntry (mediaId: $id, progress: $progress, status: CURRENT) {
		id
	}
}`
