package inject

import _ "embed"

//go:embed session_avatar_sse_tee.js
var SessionAvatarSseTeeEarlyJS string

//go:embed session_avatar.js
var SessionAvatarRuntimeJS string

//go:embed session_avatar_paint.js
var SessionAvatarPaintRuntimeJS string
