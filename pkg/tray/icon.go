package tray

import _ "embed"

// Icon is embedded from icon.ico file at compile time
// IMPORTANT: Windows system tray requires ICO format, not PNG!
//
// To customize the icon:
// 1. Download an icon (SVG or PNG) from:
//    - https://icons8.com/icons/set/server
//    - https://www.flaticon.com/free-icons/server
//    - https://fontawesome.com/icons/server
// 2. Convert to ICO format (16x16 or 32x32) using:
//    - https://redketchup.io/icon-converter
//    - https://convertico.com/
//    - https://icoconvert.com/
// 3. Save as pkg/tray/icon.ico (replacing the existing file)
// 4. Rebuild the application
//
//go:embed icon.ico
var Icon []byte
