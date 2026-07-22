package routes

import (
	"river-api/internal/handlers"
	"river-api/internal/middleware"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Register(r *gin.Engine, secret string,
	auth *handlers.AuthHandler,
	library *handlers.LibraryHandler,
	movie *handlers.MovieHandler,
	tvshow *handlers.TVShowHandler,
	music *handlers.MusicHandler,
	audiobook *handlers.AudiobookHandler,
	admin *handlers.AdminHandler,
	adminUsers *handlers.AdminUsersHandler,
	upload *handlers.UploadHandler,
	progress *handlers.ProgressHandler,
	progressWS *handlers.ProgressWSHandler,
	recentlyAdded *handlers.RecentlyAddedHandler,
	credits *handlers.CreditsHandler,
	search *handlers.SearchHandler,
	subtitle *handlers.SubtitleHandler,
	audioTrack *handlers.AudioTrackHandler,
	collection *handlers.CollectionHandler,
	watchlist *handlers.WatchlistHandler,
	watchParty *handlers.WatchPartyHandler,
	serviceLog *handlers.ServiceLogHandler,
	request    *handlers.RequestHandler,
	imageProxy *handlers.ImageProxyHandler,
) {
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	// Swagger UI at /swagger/index.html, raw spec at /swagger/doc.json.
	// Registered before the auth middleware on the /api group so the docs
	// page is reachable without logging in (it has its own Authorize
	// button that lets you paste a bearer token for try-it-out calls).
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")

	// Auth (public)
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", auth.Register)
		authGroup.POST("/login", auth.Login)
		authGroup.POST("/refresh", auth.Refresh)
		authGroup.POST("/logout", auth.Logout)
	}

	// Public image proxy — requires no auth because the upstreams it
	// fetches (currently just image.tmdb.org) are themselves public
	// CDNs. Gated by the host allowlist inside the handler so it
	// can't be turned into an SSRF.
	api.GET("/image", imageProxy.Get)

	// Authenticated routes
	protected := api.Group("", middleware.Auth(secret))
	{
		protected.GET("/auth/me", auth.Me)
		protected.PUT("/auth/me", auth.UpdateMe)
		protected.POST("/auth/me/password", auth.ChangePassword)

		// Logs
		protected.POST("/logs", serviceLog.Create)
		protected.GET("/admin/logs", middleware.AdminOnly(), serviceLog.List)

		// Admin
		protected.GET("/admin/stats", middleware.AdminOnly(), admin.GetStats)
		protected.GET("/admin/active-sessions", middleware.AdminOnly(), progress.ActiveSessions)
		protected.POST("/admin/scan", middleware.AdminOnly(), admin.TriggerScan)
		protected.POST("/admin/requeue-untranscoded", middleware.AdminOnly(), admin.RequeueUntranscoded)
		protected.GET("/admin/scanner-state", middleware.AdminOnly(), admin.ScannerState)
		protected.POST("/admin/scanner-state/forget", middleware.AdminOnly(), admin.ForgetScannerState)
		protected.POST("/admin/upload", middleware.AdminOnly(), upload.Upload)
		protected.POST("/movies/:id/refresh-metadata", middleware.AdminOnly(), admin.RefreshMovieMetadata)
		protected.POST("/movies/:id/identify", middleware.AdminOnly(), admin.IdentifyMovie)
		protected.POST("/tvshows/:id/refresh-metadata", middleware.AdminOnly(), admin.RefreshTVShowMetadata)
		protected.POST("/tvshows/:id/identify", middleware.AdminOnly(), admin.IdentifyTVShow)
		protected.GET("/admin/unidentified", middleware.AdminOnly(), admin.Unidentified)
		protected.POST("/audiobooks/:id/refresh-metadata", middleware.AdminOnly(), admin.RefreshAudiobookMetadata)
		protected.POST("/artists/:id/refresh-metadata", middleware.AdminOnly(), admin.RefreshArtistMetadata)

		// Admin — user management
		adminUsersGrp := protected.Group("/admin/users", middleware.AdminOnly())
		{
			adminUsersGrp.GET("", adminUsers.ListUsers)
			adminUsersGrp.POST("", adminUsers.CreateUser)
			adminUsersGrp.PUT("/:id", adminUsers.UpdateUser)
			adminUsersGrp.POST("/:id/set-password", adminUsers.SetPassword)
			adminUsersGrp.DELETE("/:id", adminUsers.DeleteUser)
			adminUsersGrp.GET("/:id/activity", adminUsers.GetActivity)
		}

		// Libraries (admin only for mutation)
		libs := protected.Group("/libraries")
		{
			libs.GET("", library.List)
			libs.GET("/:id", library.Get)
			libs.POST("", middleware.AdminOnly(), library.Create)
			libs.PUT("/:id", middleware.AdminOnly(), library.Update)
			libs.DELETE("/:id", middleware.AdminOnly(), library.Delete)
		}

		// Movies
		movies := protected.Group("/movies")
		{
			movies.GET("", movie.List)
			movies.GET("/:id", movie.Get)
			movies.GET("/:id/similar", movie.Similar)
			movies.GET("/:id/stream", movie.Stream)
			movies.GET("/:id/download", movie.Download)
			movies.GET("/:id/credits", credits.GetMovieCredits)
			movies.PUT("/:id/credits", middleware.AdminOnly(), credits.SetMovieCredits)
			movies.GET("/:id/subtitles", subtitle.ListMovieSubtitles)
			movies.GET("/:id/audio-tracks", audioTrack.ListMovieAudioTracks)
			movies.POST("", middleware.AdminOnly(), movie.Create)
			movies.PUT("/:id", middleware.AdminOnly(), movie.Update)
			movies.PATCH("/:id/file-path", middleware.AdminOnly(), movie.UpdateFilePath)
			movies.PATCH("/:id/source-path", middleware.AdminOnly(), movie.UpdateSourcePath)
			movies.DELETE("/:id", middleware.AdminOnly(), movie.Delete)
		}

		// TV Shows
		shows := protected.Group("/tvshows")
		{
			shows.GET("", tvshow.ListShows)
			shows.GET("/:id", tvshow.GetShow)
			shows.GET("/:id/similar", tvshow.Similar)
			shows.GET("/:id/next-episode", progress.NextEpisode)
			shows.GET("/:id/credits", credits.GetTVShowCredits)
			shows.PUT("/:id/credits", middleware.AdminOnly(), credits.SetTVShowCredits)
			shows.POST("", middleware.AdminOnly(), tvshow.CreateShow)
			shows.PUT("/:id", middleware.AdminOnly(), tvshow.UpdateShow)
			shows.PATCH("/:id/folder-path", middleware.AdminOnly(), tvshow.UpdateFolderPath)
			shows.DELETE("/:id", middleware.AdminOnly(), tvshow.DeleteShow)

			shows.GET("/:id/seasons", tvshow.ListSeasons)
			shows.POST("/:id/seasons", middleware.AdminOnly(), tvshow.CreateSeason)
			shows.PUT("/:id/seasons/:seasonId", middleware.AdminOnly(), tvshow.UpdateSeason)

			shows.GET("/:id/seasons/:seasonId/episodes", tvshow.ListEpisodes)
			shows.POST("/:id/seasons/:seasonId/episodes", middleware.AdminOnly(), tvshow.CreateEpisode)
			shows.PUT("/:id/seasons/:seasonId/episodes/:episodeId", middleware.AdminOnly(), tvshow.UpdateEpisode)
			shows.DELETE("/:id/seasons/:seasonId/episodes/:episodeId", middleware.AdminOnly(), tvshow.DeleteEpisode)
			shows.PATCH("/:id/seasons/:seasonId/episodes/:episodeId/source-path", middleware.AdminOnly(), tvshow.UpdateEpisodeSourcePath)
			shows.GET("/:id/seasons/:seasonId/episodes/:episodeId/stream", tvshow.StreamEpisode)
			shows.GET("/:id/seasons/:seasonId/episodes/:episodeId/download", tvshow.DownloadEpisode)
			shows.GET("/:id/seasons/:seasonId/episodes/:episodeId/subtitles", subtitle.ListEpisodeSubtitles)
			shows.GET("/:id/seasons/:seasonId/episodes/:episodeId/audio-tracks", audioTrack.ListEpisodeAudioTracks)
		}

		// Music — Artists
		artists := protected.Group("/artists")
		{
			artists.GET("", music.ListArtists)
			artists.GET("/:id", music.GetArtist)
			artists.GET("/:id/albums", music.ListArtistAlbums)
			artists.POST("", middleware.AdminOnly(), music.CreateArtist)
			artists.PUT("/:id", middleware.AdminOnly(), music.UpdateArtist)
			artists.DELETE("/:id", middleware.AdminOnly(), music.DeleteArtist)
		}

		// Music — Albums
		albums := protected.Group("/albums")
		{
			albums.GET("", music.ListAlbums)
			albums.GET("/:id", music.GetAlbum)
			albums.GET("/:id/tracks", music.ListAlbumTracks)
			albums.POST("", middleware.AdminOnly(), music.CreateAlbum)
			albums.PUT("/:id", middleware.AdminOnly(), music.UpdateAlbum)
			albums.DELETE("/:id", middleware.AdminOnly(), music.DeleteAlbum)
		}

		// Music — Tracks
		tracks := protected.Group("/tracks")
		{
			tracks.GET("/:id", music.GetTrack)
			tracks.GET("/:id/stream", music.StreamTrack)
			tracks.POST("", middleware.AdminOnly(), music.CreateTrack)
			tracks.DELETE("/:id", middleware.AdminOnly(), music.DeleteTrack)
		}

		// Subtitles
		protected.GET("/subtitles/:id/stream", subtitle.Stream)
		protected.POST("/subtitles", middleware.AdminOnly(), subtitle.Create)
		protected.DELETE("/subtitles/:id", middleware.AdminOnly(), subtitle.Delete)

		// Audio tracks
		protected.GET("/audio-tracks/:id/stream", audioTrack.Stream)
		protected.POST("/audio-tracks", middleware.AdminOnly(), audioTrack.Create)
		protected.DELETE("/audio-tracks/:id", middleware.AdminOnly(), audioTrack.Delete)

		// People
		protected.GET("/people/:id", credits.GetPerson)

		// Search
		protected.GET("/search", search.Search)

		// Recently added
		protected.GET("/recently-added", recentlyAdded.List)

		// Watch progress — registered with explicit paths rather than an
		// empty-path group child (`prog.GET("", h)`), which has tripped over
		// Gin trie-node edge cases when a sibling sub-path like /ws is
		// declared first.
		protected.GET("/progress/ws", progressWS.ServeWS)
		protected.GET("/progress", progress.Get)
		protected.DELETE("/progress", progress.Delete)
		protected.PUT("/progress/completed", progress.SetCompleted)
		protected.PUT("/progress/show-completed", progress.SetShowCompleted)
		protected.GET("/progress/show-states", progress.ShowStates)
		protected.GET("/progress/show-state", progress.ShowState)
		protected.GET("/progress/continue-watching", progress.ContinueWatching)
		protected.GET("/progress/next-up", progress.NextUp)
		protected.POST("/progress/next-up/:episode_id/dismiss", progress.DismissNextUp)
		protected.DELETE("/progress/next-up/:episode_id/dismiss", progress.UndismissNextUp)
		protected.GET("/progress/all", progress.GetAll)

		// Collections
		cols := protected.Group("/collections")
		{
			cols.GET("", collection.List)
			cols.POST("", collection.Create)
			cols.GET("/:id", collection.Get)
			cols.PUT("/:id", collection.Update)
			cols.DELETE("/:id", collection.Delete)
			cols.POST("/:id/items", collection.AddItem)
			cols.DELETE("/:id/items/:itemId", collection.RemoveItem)
		}

		// Audiobooks
		audiobooks := protected.Group("/audiobooks")
		{
			audiobooks.GET("", audiobook.List)
			audiobooks.GET("/:id", audiobook.Get)
			audiobooks.GET("/:id/similar", audiobook.Similar)
			audiobooks.GET("/:id/chapters", audiobook.ListChapters)
			audiobooks.GET("/:id/chapters/:chapterId/stream", audiobook.StreamChapter)
			audiobooks.POST("", middleware.AdminOnly(), audiobook.Create)
			audiobooks.PUT("/:id", middleware.AdminOnly(), audiobook.Update)
			audiobooks.DELETE("/:id", middleware.AdminOnly(), audiobook.Delete)
			audiobooks.POST("/:id/chapters", middleware.AdminOnly(), audiobook.CreateChapter)
		}

		// Watchlist
		wl := protected.Group("/watchlist")
		{
			wl.GET("", watchlist.List)
			wl.POST("", watchlist.Add)
			wl.DELETE("/:id", watchlist.Remove)
		}

		// Watch parties
		wp := protected.Group("/watchparty")
		{
			wp.POST("", watchParty.Create)
			wp.GET("/:id", watchParty.Get)
			wp.DELETE("/:id", watchParty.Delete)
			wp.GET("/:id/ws", watchParty.ServeWS)
		}

		// Requests (Radarr / Sonarr)
		req := protected.Group("/request")
		{
			req.GET("/movies", request.SearchMovies)
			req.POST("/movies", request.AddMovie)
			req.GET("/shows", request.SearchShows)
			req.POST("/shows", request.AddShow)
			req.GET("/calendar", request.Calendar)
		}
	}
}
