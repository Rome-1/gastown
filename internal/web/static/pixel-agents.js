(function() {
    'use strict';

    // ============================================
    // PIXEL AGENTS - Procedural sprite engine
    // ============================================
    // Draws animated pixel art characters on <canvas> elements
    // based on Gas Town agent state (working, idle, stale, stuck, zombie).

    var SCALE = 2;       // Render scale (each pixel = 2x2 CSS pixels)
    var SPRITE_W = 16;   // Sprite width in logical pixels
    var SPRITE_H = 16;   // Sprite height in logical pixels
    var FRAME_MS = 150;  // Animation frame duration (fast & snappy)

    // Color palettes for different agent "skins" (deterministic from name hash)
    var PALETTES = [
        { body: '#4a9eff', shirt: '#2d6bc4', hair: '#2c2c3e', accent: '#7bc0ff' },  // blue
        { body: '#50c878', shirt: '#3a9a5b', hair: '#3e2c2c', accent: '#7aeaa0' },  // green
        { body: '#ff6b6b', shirt: '#c44a4a', hair: '#2c2c3e', accent: '#ff9e9e' },  // red
        { body: '#ffa94d', shirt: '#c4842d', hair: '#1a1a2e', accent: '#ffc97d' },  // orange
        { body: '#cc5de8', shirt: '#9a3db5', hair: '#2c2c3e', accent: '#de8ef5' },  // purple
        { body: '#20c997', shirt: '#18967a', hair: '#3e3e2c', accent: '#5eeabd' },  // teal
        { body: '#ffd43b', shirt: '#c4a42d', hair: '#2c1a1a', accent: '#ffe066' },  // yellow
        { body: '#74c0fc', shirt: '#5690c4', hair: '#2c2c3e', accent: '#a5d8ff' },  // light blue
    ];

    // Skin tone for face/hands
    var SKIN = '#f0c8a0';
    var SKIN_SHADOW = '#d4a878';
    var EYE_COLOR = '#1a1a2e';
    var DESK_COLOR = '#6b5b4f';
    var DESK_TOP = '#8b7b6f';
    var SCREEN_COLOR = '#0d1a2a';
    var SCREEN_GLOW = '#4a9eff';
    var SCREEN_GREEN = '#3fb950';
    var SCREEN_DIM = '#2a4a6a';
    var MONITOR = '#444';
    var MONITOR_EDGE = '#555';
    var CHAIR_COLOR = '#3a3a4a';
    var CHAIR_DARK = '#2a2a3a';
    var FLOOR = '#1a1a22';

    function hashName(name) {
        var h = 0;
        for (var i = 0; i < name.length; i++) {
            h = ((h << 5) - h + name.charCodeAt(i)) | 0;
        }
        return Math.abs(h);
    }

    function getPalette(name) {
        return PALETTES[hashName(name) % PALETTES.length];
    }

    // Draw a single pixel (scaled)
    function px(ctx, x, y, color) {
        ctx.fillStyle = color;
        ctx.fillRect(x * SCALE, y * SCALE, SCALE, SCALE);
    }

    // Draw a rect of pixels
    function rect(ctx, x, y, w, h, color) {
        ctx.fillStyle = color;
        ctx.fillRect(x * SCALE, y * SCALE, w * SCALE, h * SCALE);
    }

    // ============================================
    // SPRITE DRAWING FUNCTIONS
    // ============================================

    // Draw character sitting at desk, actively typing with screen activity
    function drawWorking(ctx, pal, frame) {
        ctx.clearRect(0, 0, SPRITE_W * SCALE, SPRITE_H * SCALE);
        var f = frame % 8; // 8-frame cycle

        // Floor
        rect(ctx, 0, 15, 16, 1, FLOOR);

        // Desk surface with slight depth
        rect(ctx, 0, 11, 16, 1, DESK_TOP);
        rect(ctx, 0, 12, 16, 1, DESK_COLOR);
        // Desk legs
        rect(ctx, 1, 13, 1, 2, DESK_COLOR);
        rect(ctx, 14, 13, 1, 2, DESK_COLOR);

        // Monitor - wider with bezel
        rect(ctx, 5, 6, 7, 1, MONITOR_EDGE);
        rect(ctx, 5, 7, 1, 3, MONITOR_EDGE);
        rect(ctx, 11, 7, 1, 3, MONITOR_EDGE);
        rect(ctx, 5, 10, 7, 1, MONITOR_EDGE);
        // Screen
        rect(ctx, 6, 7, 5, 3, SCREEN_COLOR);
        // Monitor stand
        rect(ctx, 7, 11, 3, 1, MONITOR);

        // Animated screen content - code scrolling effect
        var screenLines = [
            [SCREEN_GLOW, SCREEN_GLOW, SCREEN_DIM, SCREEN_GLOW, SCREEN_DIM],
            [SCREEN_DIM, SCREEN_GLOW, SCREEN_GLOW, SCREEN_DIM, SCREEN_GLOW],
            [SCREEN_GREEN, SCREEN_DIM, SCREEN_GLOW, SCREEN_GLOW, SCREEN_DIM],
            [SCREEN_GLOW, SCREEN_DIM, SCREEN_GREEN, SCREEN_DIM, SCREEN_GLOW],
        ];
        for (var row = 0; row < 3; row++) {
            var lineIdx = (row + f) % screenLines.length;
            var line = screenLines[lineIdx];
            for (var col = 0; col < 5; col++) {
                px(ctx, 6 + col, 7 + row, line[col]);
            }
        }
        // Blinking cursor
        if (f % 3 !== 0) {
            px(ctx, 6 + (f % 4), 9, '#fff');
        }

        // Chair
        rect(ctx, 0, 7, 1, 1, CHAIR_COLOR);
        rect(ctx, 0, 8, 1, 4, CHAIR_DARK);
        rect(ctx, 1, 12, 1, 3, CHAIR_DARK);
        px(ctx, 0, 14, CHAIR_DARK);

        // Character - sitting, facing right toward monitor
        // Hair
        rect(ctx, 2, 3, 3, 1, pal.hair);
        rect(ctx, 2, 4, 3, 1, pal.hair);
        px(ctx, 4, 3, pal.hair);

        // Face (side profile)
        rect(ctx, 2, 5, 3, 2, SKIN);
        px(ctx, 4, 4, SKIN);
        // Eye
        px(ctx, 4, 5, EYE_COLOR);
        // Mouth (subtle)
        px(ctx, 4, 6, SKIN_SHADOW);

        // Body
        rect(ctx, 1, 7, 4, 1, pal.shirt);
        rect(ctx, 1, 8, 4, 2, pal.body);
        rect(ctx, 1, 10, 3, 1, pal.body);

        // Arms - animated typing motion
        // Upper arm
        px(ctx, 4, 8, SKIN);
        px(ctx, 4, 9, SKIN);

        // Typing hands - 4 distinct positions
        var handPos = f % 4;
        if (handPos === 0) {
            px(ctx, 5, 10, SKIN);
            px(ctx, 6, 10, SKIN);
        } else if (handPos === 1) {
            px(ctx, 5, 10, SKIN_SHADOW);
            px(ctx, 6, 10, SKIN);
            px(ctx, 7, 10, SKIN);
        } else if (handPos === 2) {
            px(ctx, 5, 10, SKIN);
            px(ctx, 7, 10, SKIN);
        } else {
            px(ctx, 5, 10, SKIN);
            px(ctx, 6, 10, SKIN_SHADOW);
        }

        // Legs (sitting)
        rect(ctx, 2, 11, 2, 1, pal.shirt);
        // Feet
        px(ctx, 2, 13, '#333');
        px(ctx, 3, 13, '#333');

        // Coffee mug on desk (little detail)
        px(ctx, 13, 10, '#c4a882');
        px(ctx, 13, 9, '#c4a882');
        px(ctx, 14, 10, '#c4a882');
        // Steam from coffee - animated
        if (f < 4) {
            px(ctx, 13, 8, 'rgba(200,200,200,0.4)');
        } else {
            px(ctx, 14, 8, 'rgba(200,200,200,0.3)');
        }

        // Subtle head bob while typing
        if (f === 2 || f === 6) {
            // Slight lean forward on some frames (already handled by static position)
        }
    }

    // Draw character standing idle with breathing animation
    function drawIdle(ctx, pal, frame) {
        ctx.clearRect(0, 0, SPRITE_W * SCALE, SPRITE_H * SCALE);
        var f = frame % 16; // longer cycle for idle

        // Floor
        rect(ctx, 0, 15, 16, 1, FLOOR);

        // Breathing cycle (subtle body shift)
        var breathe = (f < 8) ? 0 : 0;
        var armSwing = Math.floor(f / 4) % 2;

        // Shadow on floor
        rect(ctx, 5, 14, 6, 1, 'rgba(0,0,0,0.2)');

        // Hair
        rect(ctx, 6, 2 + breathe, 4, 2, pal.hair);
        px(ctx, 5, 3 + breathe, pal.hair);
        px(ctx, 10, 3 + breathe, pal.hair);

        // Face
        rect(ctx, 6, 4 + breathe, 4, 2, SKIN);
        px(ctx, 5, 4 + breathe, SKIN);
        px(ctx, 10, 4 + breathe, SKIN);

        // Eyes - slow blink cycle
        if (f === 12 || f === 13) {
            // Blink
            px(ctx, 7, 4 + breathe, SKIN_SHADOW);
            px(ctx, 9, 4 + breathe, SKIN_SHADOW);
        } else {
            px(ctx, 7, 4 + breathe, EYE_COLOR);
            px(ctx, 9, 4 + breathe, EYE_COLOR);
        }

        // Mouth
        px(ctx, 8, 5 + breathe, SKIN_SHADOW);

        // Body / shirt
        rect(ctx, 6, 6 + breathe, 4, 1, pal.shirt);
        rect(ctx, 5, 7 + breathe, 6, 1, pal.shirt);
        rect(ctx, 6, 8 + breathe, 4, 2, pal.body);

        // Arms - gentle idle sway
        if (armSwing === 0) {
            px(ctx, 5, 7, SKIN);
            px(ctx, 5, 8, SKIN);
            px(ctx, 10, 7, SKIN);
            px(ctx, 10, 8, SKIN);
        } else {
            px(ctx, 5, 7, SKIN);
            px(ctx, 4, 8, SKIN);
            px(ctx, 10, 7, SKIN);
            px(ctx, 11, 8, SKIN);
        }

        // Belt
        rect(ctx, 6, 10, 4, 1, pal.shirt);

        // Legs
        rect(ctx, 6, 11, 2, 2, pal.body);
        rect(ctx, 8, 11, 2, 2, pal.body);

        // Feet
        px(ctx, 5, 13, '#333');
        rect(ctx, 6, 13, 2, 1, '#333');
        rect(ctx, 8, 13, 2, 1, '#333');
        px(ctx, 10, 13, '#333');

        // Occasional look around (turn head slightly)
        if (f >= 8 && f <= 10) {
            // Looking right
            px(ctx, 10, 4 + breathe, SKIN);
        }
    }

    // Draw stale agent - nodding off at desk with Z's
    function drawStale(ctx, pal, frame) {
        ctx.clearRect(0, 0, SPRITE_W * SCALE, SPRITE_H * SCALE);
        var f = frame % 12;

        // Floor
        rect(ctx, 0, 15, 16, 1, FLOOR);

        // Desk
        rect(ctx, 0, 11, 16, 1, DESK_TOP);
        rect(ctx, 0, 12, 16, 1, DESK_COLOR);
        rect(ctx, 1, 13, 1, 2, DESK_COLOR);
        rect(ctx, 14, 13, 1, 2, DESK_COLOR);

        // Monitor (screen off/dim)
        rect(ctx, 5, 6, 7, 1, MONITOR_EDGE);
        rect(ctx, 5, 7, 1, 3, MONITOR_EDGE);
        rect(ctx, 11, 7, 1, 3, MONITOR_EDGE);
        rect(ctx, 5, 10, 7, 1, MONITOR_EDGE);
        rect(ctx, 6, 7, 5, 3, '#0a0f18'); // dim screen
        // Screensaver dot bouncing
        var dotX = 6 + (f % 5);
        var dotY = 7 + (f % 3);
        px(ctx, dotX, dotY, SCREEN_DIM);
        rect(ctx, 7, 11, 3, 1, MONITOR);

        // Chair
        rect(ctx, 0, 7, 1, 1, CHAIR_COLOR);
        rect(ctx, 0, 8, 1, 4, CHAIR_DARK);
        rect(ctx, 1, 12, 1, 3, CHAIR_DARK);
        px(ctx, 0, 14, CHAIR_DARK);

        // Character - head drooping cycle
        var headDroop = (f < 4) ? 0 : (f < 8) ? 1 : 0;

        // Hair
        rect(ctx, 2, 3 + headDroop, 3, 1, pal.hair);
        rect(ctx, 2, 4 + headDroop, 3, 1, pal.hair);

        // Face - eyes closed
        rect(ctx, 2, 5 + headDroop, 3, 2, SKIN);
        px(ctx, 4, 5 + headDroop, SKIN_SHADOW); // closed eye
        px(ctx, 4, 6 + headDroop, SKIN_SHADOW); // mouth open (yawn)

        // Body
        rect(ctx, 1, 7, 4, 1, pal.shirt);
        rect(ctx, 1, 8, 4, 2, pal.body);
        rect(ctx, 1, 10, 3, 1, pal.body);

        // Arms resting on desk
        px(ctx, 4, 9, SKIN);
        px(ctx, 5, 10, SKIN);
        px(ctx, 6, 10, SKIN);

        // Legs
        rect(ctx, 2, 11, 2, 1, pal.shirt);
        px(ctx, 2, 13, '#333');
        px(ctx, 3, 13, '#333');

        // Floating Z's - multi-layered, rising animation
        var z1y = 2 - Math.floor(f / 3);
        var z2y = 3 - Math.floor(((f + 4) % 12) / 3);
        var z3y = 1 - Math.floor(((f + 8) % 12) / 4);

        // Big Z
        if (z1y >= -1 && z1y <= 3) {
            px(ctx, 12, z1y + 1, '#8b949e');
            px(ctx, 13, z1y, '#8b949e');
            px(ctx, 14, z1y + 1, '#8b949e');
            px(ctx, 12, z1y, '#8b949e');
            px(ctx, 14, z1y, '#8b949e');
        }
        // Small z
        if (z2y >= 0 && z2y <= 3) {
            px(ctx, 11, z2y, '#6e7681');
            px(ctx, 12, z2y, '#6e7681');
        }
        // Tiny z
        if (z3y >= 0 && z3y <= 2) {
            px(ctx, 13, z3y, '#555');
        }

        // Coffee mug (cold, no steam)
        px(ctx, 13, 10, '#8a7a6e');
        px(ctx, 14, 10, '#8a7a6e');
    }

    // Draw stuck agent - panicking with flashing alert
    function drawStuck(ctx, pal, frame) {
        ctx.clearRect(0, 0, SPRITE_W * SCALE, SPRITE_H * SCALE);
        var f = frame % 8;

        // Floor
        rect(ctx, 0, 15, 16, 1, FLOOR);

        // Shadow (jittery)
        var jitterX = (f % 2 === 0) ? 0 : 1;
        rect(ctx, 5 + jitterX, 14, 6, 1, 'rgba(0,0,0,0.2)');

        // Character - agitated standing pose
        var bounce = (f % 2 === 0) ? 0 : -1;

        // Hair (messy from stress)
        rect(ctx, 6, 2 + bounce, 4, 2, pal.hair);
        px(ctx, 5, 2 + bounce, pal.hair); // sticking up
        px(ctx, 10, 2 + bounce, pal.hair);
        px(ctx, 7, 1 + bounce, pal.hair); // hair standing on end

        // Face - distressed
        rect(ctx, 6, 4 + bounce, 4, 2, SKIN);
        px(ctx, 5, 4 + bounce, SKIN);
        px(ctx, 10, 4 + bounce, SKIN);

        // Wide eyes
        px(ctx, 7, 4 + bounce, '#fff');
        px(ctx, 9, 4 + bounce, '#fff');
        // Pupils (darting)
        if (f < 4) {
            px(ctx, 7, 4 + bounce, EYE_COLOR);
            px(ctx, 9, 4 + bounce, EYE_COLOR);
        }

        // Open mouth (surprise)
        px(ctx, 8, 5 + bounce, '#c44');

        // Body - jittery
        rect(ctx, 6, 6 + bounce, 4, 1, pal.shirt);
        rect(ctx, 5, 7 + bounce, 6, 1, pal.shirt);
        rect(ctx, 6, 8 + bounce, 4, 2, pal.body);

        // Arms raised in distress
        if (f < 4) {
            px(ctx, 4, 6 + bounce, SKIN);
            px(ctx, 4, 5 + bounce, SKIN);
            px(ctx, 11, 6 + bounce, SKIN);
            px(ctx, 11, 5 + bounce, SKIN);
        } else {
            px(ctx, 4, 6 + bounce, SKIN);
            px(ctx, 3, 5 + bounce, SKIN);
            px(ctx, 11, 6 + bounce, SKIN);
            px(ctx, 12, 5 + bounce, SKIN);
        }

        // Belt
        rect(ctx, 6, 10 + bounce, 4, 1, pal.shirt);

        // Legs - pacing
        if (f < 2) {
            rect(ctx, 6, 11, 2, 2, pal.body);
            rect(ctx, 8, 11, 2, 2, pal.body);
        } else if (f < 4) {
            rect(ctx, 5, 11, 2, 2, pal.body);
            rect(ctx, 9, 11, 2, 2, pal.body);
        } else if (f < 6) {
            rect(ctx, 6, 11, 2, 2, pal.body);
            rect(ctx, 8, 11, 2, 2, pal.body);
        } else {
            rect(ctx, 7, 11, 2, 2, pal.body);
            rect(ctx, 9, 11, 2, 2, pal.body);
        }

        // Feet
        px(ctx, 5, 13, '#333');
        rect(ctx, 6, 13, 4, 1, '#333');
        px(ctx, 10, 13, '#333');

        // Alert bubble - flashing red/yellow
        var alertColor1 = (f < 4) ? '#f85149' : '#ffd43b';
        var alertColor2 = (f < 4) ? '#ffd43b' : '#f85149';

        // Bubble background
        rect(ctx, 12, 0, 4, 4, alertColor1);
        // "!" exclamation
        px(ctx, 14, 0, '#fff');
        px(ctx, 14, 1, '#fff');
        px(ctx, 14, 2, '#fff');
        px(ctx, 14, 3, '#fff');
        px(ctx, 13, 1, alertColor2);
        // Bubble pointer
        px(ctx, 12, 4, alertColor1);

        // Sweat drops
        if (f === 1 || f === 5) {
            px(ctx, 5, 3 + bounce, '#74c0fc');
        }
        if (f === 3 || f === 7) {
            px(ctx, 11, 3 + bounce, '#74c0fc');
        }
    }

    // Draw zombie agent - ghost rising from desk
    function drawZombie(ctx, pal, frame) {
        ctx.clearRect(0, 0, SPRITE_W * SCALE, SPRITE_H * SCALE);
        var f = frame % 16;

        // Floor
        rect(ctx, 0, 15, 16, 1, FLOOR);

        // Floating sine-wave bob
        var floatY = Math.round(Math.sin(f * 0.4) * 1.5);
        var baseY = 2 + floatY;

        // Ghostly trail / afterimage
        var trailAlpha = 0.15;
        ctx.fillStyle = 'rgba(139, 148, 158, ' + trailAlpha + ')';
        ctx.fillRect(6 * SCALE, (baseY + 8) * SCALE, 4 * SCALE, 3 * SCALE);

        // Ghost body - ethereal, semi-transparent
        var ghostAlpha = 0.5 + Math.sin(f * 0.3) * 0.15;

        // Shroud / robe shape
        var ghostColor = 'rgba(180, 190, 200, ' + ghostAlpha + ')';
        var ghostLight = 'rgba(220, 230, 240, ' + (ghostAlpha + 0.1) + ')';
        var ghostDark = 'rgba(100, 110, 130, ' + ghostAlpha + ')';

        // Head
        rect(ctx, 6, baseY, 4, 2, ghostLight);
        px(ctx, 5, baseY + 1, ghostLight);
        px(ctx, 10, baseY + 1, ghostLight);

        // Eyes - hollow, glowing
        var eyeGlow = (f % 4 < 2) ? 'rgba(100, 200, 255, 0.7)' : 'rgba(80, 160, 220, 0.5)';
        px(ctx, 7, baseY, eyeGlow);
        px(ctx, 9, baseY, eyeGlow);

        // Body - flowing robe
        rect(ctx, 5, baseY + 2, 6, 4, ghostColor);
        rect(ctx, 6, baseY + 6, 4, 2, ghostColor);

        // Robe highlights
        px(ctx, 6, baseY + 2, ghostLight);
        px(ctx, 9, baseY + 3, ghostLight);

        // Wispy bottom edge - animated wave
        for (var wx = 0; wx < 6; wx++) {
            var waveY = baseY + 8 + Math.round(Math.sin((f + wx * 2) * 0.5) * 0.8);
            if (waveY >= 0 && waveY < 16) {
                px(ctx, 5 + wx, waveY, ghostDark);
            }
        }

        // Arms - spectral, reaching slightly
        var armWave = Math.round(Math.sin(f * 0.5) * 0.8);
        px(ctx, 4, baseY + 3 + armWave, ghostDark);
        px(ctx, 3, baseY + 4 + armWave, ghostDark);
        px(ctx, 11, baseY + 3 - armWave, ghostDark);
        px(ctx, 12, baseY + 4 - armWave, ghostDark);

        // Sparkle particles floating around
        var sparkles = [
            [(f * 3 + 7) % 16, (f * 2 + 3) % 14],
            [(f * 5 + 2) % 16, (f * 3 + 8) % 14],
            [(f * 7 + 11) % 16, (f + 5) % 14],
        ];
        for (var si = 0; si < sparkles.length; si++) {
            var sx = sparkles[si][0];
            var sy = sparkles[si][1];
            var sparkAlpha = 0.3 + Math.sin(f * 0.8 + si) * 0.2;
            px(ctx, sx, sy, 'rgba(150, 200, 255, ' + sparkAlpha + ')');
        }

        // Tombstone/broken monitor at bottom
        rect(ctx, 1, 12, 3, 3, '#3a3a4a');
        rect(ctx, 1, 11, 3, 1, '#4a4a5a');
        px(ctx, 2, 12, '#555');
        px(ctx, 2, 13, '#555');
    }

    // ============================================
    // STATE MACHINE
    // ============================================

    var DRAW_FN = {
        'working': drawWorking,
        'idle':    drawIdle,
        'stale':   drawStale,
        'stuck':   drawStuck,
        'zombie':  drawZombie
    };

    // Active agents: { canvasId -> { ctx, pal, state, frame, name } }
    var agents = {};
    var animTimer = null;

    function tick() {
        for (var id in agents) {
            var a = agents[id];
            a.frame++;
            var drawFn = DRAW_FN[a.state] || drawIdle;
            drawFn(a.ctx, a.pal, a.frame);
        }
    }

    function startAnimation() {
        if (animTimer) return;
        animTimer = setInterval(tick, FRAME_MS);
    }

    function stopAnimation() {
        if (animTimer) {
            clearInterval(animTimer);
            animTimer = null;
        }
    }

    // ============================================
    // PUBLIC API
    // ============================================

    // Initialize a canvas for an agent
    function initAgent(canvas, name, state) {
        if (!canvas) return;

        var id = canvas.id || name;
        canvas.width = SPRITE_W * SCALE;
        canvas.height = SPRITE_H * SCALE;
        var ctx = canvas.getContext('2d');
        var pal = getPalette(name);

        agents[id] = {
            ctx: ctx,
            pal: pal,
            state: state || 'idle',
            frame: 0,
            name: name
        };

        // Draw first frame immediately
        var drawFn = DRAW_FN[agents[id].state] || drawIdle;
        drawFn(ctx, pal, 0);

        startAnimation();
    }

    // Update agent state
    function updateAgentState(canvasId, newState) {
        var a = agents[canvasId];
        if (!a) return;
        if (a.state === newState) return;
        a.state = newState;
        a.frame = 0;
    }

    // Initialize all pixel-agent canvases on the page
    function initAll() {
        var canvases = document.querySelectorAll('canvas[data-pixel-agent]');
        for (var i = 0; i < canvases.length; i++) {
            var c = canvases[i];
            var name = c.getAttribute('data-pixel-agent');
            var state = c.getAttribute('data-pixel-state') || 'idle';
            initAgent(c, name, state);
        }
    }

    // Re-sync all canvases (called after HTMX morph/swap)
    function resyncAll() {
        var canvases = document.querySelectorAll('canvas[data-pixel-agent]');
        var seen = {};

        for (var i = 0; i < canvases.length; i++) {
            var c = canvases[i];
            var name = c.getAttribute('data-pixel-agent');
            var state = c.getAttribute('data-pixel-state') || 'idle';
            var id = c.id || name;
            seen[id] = true;

            if (agents[id]) {
                // Agent exists - update state if changed, re-attach ctx if canvas was replaced
                agents[id].ctx = c.getContext('2d');
                c.width = SPRITE_W * SCALE;
                c.height = SPRITE_H * SCALE;
                if (agents[id].state !== state) {
                    agents[id].state = state;
                    agents[id].frame = 0;
                }
                // Redraw immediately
                var drawFn = DRAW_FN[state] || drawIdle;
                drawFn(agents[id].ctx, agents[id].pal, agents[id].frame);
            } else {
                initAgent(c, name, state);
            }
        }

        // Remove agents whose canvases no longer exist
        for (var aid in agents) {
            if (!seen[aid]) {
                delete agents[aid];
            }
        }

        // Stop timer if no agents
        if (Object.keys(agents).length === 0) {
            stopAnimation();
        }
    }

    // Expose API
    window.PixelAgents = {
        init: initAll,
        resync: resyncAll,
        initAgent: initAgent,
        updateState: updateAgentState
    };

    // Auto-init on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initAll);
    } else {
        initAll();
    }
})();
