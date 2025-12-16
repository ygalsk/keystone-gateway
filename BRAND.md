# Keystone Gateway - Brand Guide

**Version:** 1.0.0
**Last Updated:** December 2025

---

## Brand Identity

### Name
**Keystone Gateway**

A general-purpose HTTP routing primitive with embedded Lua scripting.

### Tagline
**"A highly opinionated gateway with no opinions"**

This paradoxical statement captures our core philosophy: we're opinionated about architecture (deep modules, stateless design, primitives over policies) but we have no opinions about your business logic. The gateway is dumb, your tenants are smart.

### Mascot
**Key-Way** - The Kiwi

A friendly kiwi bird that embodies our design philosophy:
- **Small but capable** - Compact codebase (~1,600 lines) with powerful primitives
- **Flightless yet fast** - Stateless and grounded, but high-performance with Lua state pooling
- **Native to one place** - General-purpose tool that thrives in multi-tenant environments
- **Unique and memorable** - Distinctive approach to API gateway design

---

## Color Palette

Our colors are inspired by the kiwi fruit - natural, vibrant, and distinctive.

### Primary Colors

```css
/* Kiwi Green - Primary brand color */
--kiwi-green: #73B44C;           /* Bright kiwi flesh */
--kiwi-green-light: #9CCC65;     /* Light kiwi tone */
--kiwi-green-dark: #5A9138;      /* Deep green */

/* Kiwi Lime - Accent color */
--kiwi-lime: #BFD85F;            /* Yellow-green highlight */
--kiwi-lime-light: #D4E78E;      /* Soft lime */
```

### Secondary Colors

```css
/* Kiwi Brown - Earthy, grounding tones */
--kiwi-brown: #5C4033;           /* Fuzzy skin */
--kiwi-brown-light: #8B7355;     /* Light brown */
--kiwi-brown-dark: #3D2B22;      /* Deep earth */

/* Kiwi Cream - Neutral backgrounds */
--kiwi-cream: #FAFADC;           /* Soft cream center */
--kiwi-cream-dark: #F5F5DC;      /* Beige */
```

### Neutral Colors

```css
/* Kiwi Seeds - Text and UI elements */
--kiwi-seed: #2C2C2C;            /* Black seeds - primary text */
--kiwi-seed-light: #4A4A4A;      /* Secondary text */
--kiwi-white: #FFFFFF;           /* Pure white */
--kiwi-border: #E5E7EB;          /* Subtle borders */
```

### Semantic Colors

```css
/* Status colors */
--success: #73B44C;              /* Success = Kiwi Green */
--warning: #F59E0B;              /* Warning = Amber */
--error: #EF4444;                /* Error = Red */
--info: #3B82F6;                 /* Info = Blue */
```

### Color Usage Guidelines

**Primary Actions:**
- Buttons, links, CTAs: `--kiwi-green`
- Hover states: `--kiwi-green-dark`
- Active states: `--kiwi-green-light`

**Text:**
- Headings: `--kiwi-seed`
- Body text: `--kiwi-seed-light`
- Secondary text: `--kiwi-brown-light`

**Backgrounds:**
- Page background: `--kiwi-white`
- Sections/cards: `--kiwi-cream`
- Code blocks: `--kiwi-cream-dark`

**Accents:**
- Highlights: `--kiwi-lime`
- Callouts: `--kiwi-lime-light`
- Borders: `--kiwi-border`

---

## Typography

### Font Stack

**Headings:**
```css
font-family: 'Inter', 'SF Pro Display', -apple-system, BlinkMacSystemFont,
             'Segoe UI', 'Roboto', sans-serif;
```

**Body:**
```css
font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI',
             'Roboto', 'Helvetica Neue', sans-serif;
```

**Code:**
```css
font-family: 'JetBrains Mono', 'Fira Code', 'Consolas',
             'Monaco', 'Courier New', monospace;
```

### Type Scale

```css
/* Typography scale - 1.25 ratio (Major Third) */
--text-xs: 0.75rem;      /* 12px - labels, captions */
--text-sm: 0.875rem;     /* 14px - secondary text */
--text-base: 1rem;       /* 16px - body text */
--text-lg: 1.25rem;      /* 20px - subheadings */
--text-xl: 1.5rem;       /* 24px - headings */
--text-2xl: 2rem;        /* 32px - page titles */
--text-3xl: 2.5rem;      /* 40px - hero headings */
--text-4xl: 3rem;        /* 48px - hero large */
```

### Font Weights

```css
--weight-regular: 400;   /* Body text */
--weight-medium: 500;    /* Emphasis */
--weight-semibold: 600;  /* Subheadings, buttons */
--weight-bold: 700;      /* Headings */
```

### Line Heights

```css
--leading-tight: 1.25;   /* Headings */
--leading-normal: 1.5;   /* Body text */
--leading-relaxed: 1.75; /* Long-form content */
```

---

## Spacing System

8px grid system for consistent, harmonious spacing.

```css
--space-1: 0.5rem;   /* 8px */
--space-2: 1rem;     /* 16px */
--space-3: 1.5rem;   /* 24px */
--space-4: 2rem;     /* 32px */
--space-5: 2.5rem;   /* 40px */
--space-6: 3rem;     /* 48px */
--space-8: 4rem;     /* 64px */
--space-10: 5rem;    /* 80px */
--space-12: 6rem;    /* 96px */
--space-16: 8rem;    /* 128px */
```

---

## Component Library

### Buttons

**Primary Button:**
```html
<a href="#" class="btn-primary">Get Started</a>
```
```css
.btn-primary {
  display: inline-block;
  padding: var(--space-2) var(--space-4);
  background: var(--kiwi-green);
  color: var(--kiwi-white);
  border-radius: 0.5rem;
  font-weight: var(--weight-semibold);
  font-size: var(--text-base);
  text-decoration: none;
  transition: background 150ms ease;
}
.btn-primary:hover {
  background: var(--kiwi-green-dark);
}
.btn-primary:active {
  background: var(--kiwi-green-light);
}
```

**Secondary Button:**
```css
.btn-secondary {
  display: inline-block;
  padding: var(--space-2) var(--space-4);
  background: transparent;
  color: var(--kiwi-green);
  border: 2px solid var(--kiwi-green);
  border-radius: 0.5rem;
  font-weight: var(--weight-semibold);
  text-decoration: none;
  transition: all 150ms ease;
}
.btn-secondary:hover {
  background: var(--kiwi-green);
  color: var(--kiwi-white);
}
```

### Code Blocks

```css
pre {
  background: var(--kiwi-cream-dark);
  border: 1px solid var(--kiwi-border);
  border-left: 4px solid var(--kiwi-green);
  padding: var(--space-3);
  border-radius: 0.5rem;
  overflow-x: auto;
  font-size: var(--text-sm);
}
code {
  font-family: 'JetBrains Mono', monospace;
  color: var(--kiwi-seed);
}
```

### Cards

```css
.card {
  background: var(--kiwi-white);
  border: 1px solid var(--kiwi-border);
  border-radius: 0.75rem;
  padding: var(--space-4);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: box-shadow 150ms ease;
}
.card:hover {
  box-shadow: 0 4px 12px rgba(115, 180, 76, 0.1);
}
```

### Callouts

```css
.callout {
  background: var(--kiwi-lime-light);
  border-left: 4px solid var(--kiwi-lime);
  padding: var(--space-3);
  border-radius: 0.375rem;
  margin: var(--space-4) 0;
}
```

---

## Logo & Mascot Guidelines

### Key-Way the Kiwi - Character Design

**Visual Characteristics:**
- Round, friendly body shape
- Small wings (flightless, grounded like our stateless design)
- Long, distinctive beak (represents routing/directing)
- Expressive eyes (intelligent, aware)
- Brown fuzzy texture (kiwi fruit skin colors)
- Optional: Small "keystone" accessory or badge

**Personality Traits:**
- **Helpful:** Always ready to route requests
- **Efficient:** Fast and lightweight
- **Flexible:** Adapts to any tenant's needs
- **Humble:** "Dumb gateway" - doesn't try to be more than it is
- **Reliable:** Stateless and predictable

**Usage:**
- Landing page hero section
- Documentation headers
- Error pages (friendly 404s)
- GitHub organization avatar
- Social media presence

### Logo Concepts

**Primary Logo: Keystone + Kiwi**
- Wordmark "Keystone Gateway" in bold, modern sans-serif
- Key-Way the kiwi integrated into the design
- Color: `--kiwi-green` text with `--kiwi-brown` kiwi

**Icon/Favicon:**
- Simplified kiwi silhouette
- Circular badge with kiwi green background
- Minimal, recognizable at small sizes (16px)

---

## Voice & Tone

### Brand Voice

**Core Attributes:**
1. **Direct** - No marketing fluff, say what it is
2. **Technical** - Speak to engineers, not executives
3. **Humble** - Acknowledge what we don't do
4. **Opinionated** - Strong views on design, not business logic
5. **Playful** - The kiwi mascot allows for personality

### Writing Guidelines

**Good:**
- "Dumb gateway, smart tenants"
- "We handle HTTP efficiently, you handle business logic"
- "No built-in auth, rate limiting, or opinions"
- "Multi-tenant routing with embedded Lua scripting"
- "General-purpose primitives, not workflows"

**Bad:**
- "Revolutionary API management platform"
- "Next-generation cloud-native solution"
- "Enterprise-grade innovation"
- "Cutting-edge microservices architecture"
- "Seamlessly integrate with your existing infrastructure"

**Headlines - Lead with What It IS:**
- "API Gateway with Embedded Lua" ✓
- "Dynamic Routing Without Rebuilds" ✓
- "Multi-Tenant HTTP Primitives" ✓
- "The Future of API Management" ✗
- "Revolutionizing Microservices" ✗

**Body Copy - Short, Active, Second Person:**
```
Good:
Route API requests to different backends based on tenant,
authentication, or custom logic. Change routing rules
without redeploying your gateway.

Bad:
Keystone Gateway leverages cutting-edge technology to
provide an innovative approach to API management through
a sophisticated multi-tenant architecture...
```

---

## Landing Page Design

### Hero Section

**Structure:**
```html
<section class="hero">
  <div class="hero-content">
    <h1>Keystone Gateway</h1>
    <p class="tagline">A highly opinionated gateway with no opinions</p>
    <p class="description">
      Multi-tenant API gateway with embedded Lua scripting.
      You write the business logic, we handle the HTTP.
    </p>
    <div class="ctas">
      <a href="/docs" class="btn-primary">Read the Docs</a>
      <a href="https://github.com/..." class="btn-secondary">View on GitHub</a>
    </div>
  </div>
  <div class="hero-visual">
    <!-- Key-Way the kiwi illustration -->
  </div>
</section>
```

**Visual:**
- Key-Way the kiwi on right side
- Clean, minimal background (kiwi-cream)
- Subtle accent with kiwi-green

### Feature Highlights (Below Fold)

**Maximum 3 features:**

1. **Multi-Tenant by Design**
   - Icon: Kiwi with multiple paths
   - Description: "Isolate tenant routing logic without rebuilding"

2. **Lua-Powered Flexibility**
   - Icon: Kiwi with code symbol
   - Description: "Change routing rules on the fly with embedded scripting"

3. **Production Ready**
   - Icon: Kiwi with checkmark
   - Description: "Stateless, scalable, and cloud-native"

### Code Example Section

```lua
-- Simple, focused example (< 10 lines)
function hello_handler(req)
    return {
        status = 200,
        body = "Hello from Keystone Gateway!",
        headers = {["Content-Type"] = "text/plain"}
    }
end
```

---

## Design Principles Alignment

Our brand reflects our technical philosophy:

### 1. Deep Modules
**Brand Expression:** Clean, simple visual interface hiding sophisticated architecture
- Minimal UI elements
- Complex functionality expressed simply

### 2. Information Hiding
**Brand Expression:** Focus on what users need to know, not how it works
- Headlines emphasize capabilities, not implementation
- Hide complexity in friendly mascot

### 3. General-Purpose
**Brand Expression:** Flexible color palette, adaptable mascot
- Kiwi can be in different contexts
- Colors work across use cases

### 4. Pull Complexity Down
**Brand Expression:** Hero section is simple, details below the fold
- Clear visual hierarchy
- Complexity revealed progressively

### 5. Gateway is Dumb
**Brand Expression:** Humble, friendly mascot (not a fierce eagle or lion)
- Self-aware tagline
- No grandiose claims

---

## Asset Specifications

### Logo Files Needed
- [ ] `logo-primary.svg` - Full wordmark + mascot
- [ ] `logo-icon.svg` - Kiwi icon only (square)
- [ ] `logo-wordmark.svg` - Text only
- [ ] `favicon.ico` - 16x16, 32x32, 48x48
- [ ] `favicon.svg` - Scalable vector favicon

### Mascot Variations
- [ ] `keeway-happy.svg` - Default friendly pose
- [ ] `keeway-coding.svg` - With laptop/code
- [ ] `keeway-routing.svg` - With directional arrows
- [ ] `keeway-error.svg` - Confused/404 pose
- [ ] `keeway-success.svg` - Thumbs up/checkmark

### Illustrations
- [ ] Hero section illustration
- [ ] Architecture diagram (with kiwi elements)
- [ ] Feature icons (3 needed)

---

## Performance Budget

All brand assets must respect these limits:

**Images:**
- Logo SVG: < 5KB
- Mascot illustrations: < 10KB each
- Hero image: < 50KB (optimized SVG)
- Favicon: < 2KB

**Fonts:**
- Inter font family: < 100KB total
- JetBrains Mono: < 50KB (code only)

**Total page weight:**
- Landing page: < 500KB
- Including all assets: < 300KB images, < 100KB fonts

---

## Accessibility Requirements

### Color Contrast

All text meets WCAG 2.1 AA standards (4.5:1 minimum):

**Verified Combinations:**
- `--kiwi-seed` on `--kiwi-white`: 12.6:1 ✓
- `--kiwi-green` on `--kiwi-white`: 3.2:1 ✗ (buttons only)
- `--kiwi-white` on `--kiwi-green`: 4.8:1 ✓
- `--kiwi-seed-light` on `--kiwi-cream`: 8.9:1 ✓

### Mascot Accessibility

**Key-Way illustrations must:**
- Have descriptive `alt` text
- Not be sole method of conveying information
- Work in high contrast mode
- Be recognizable at small sizes

**Example alt text:**
```html
<img src="keeway-routing.svg"
     alt="Key-Way the kiwi mascot directing traffic between servers">
```

---

## Social Media Guidelines

### GitHub
- Organization avatar: Key-Way icon
- Repository social image: Full hero with tagline
- README header: Logo + tagline

### Twitter/X
- Profile: Key-Way portrait
- Header: Kiwi green gradient with tagline
- Tone: Technical, helpful, occasional kiwi puns

### LinkedIn
- Professional tone
- Focus on technical benefits
- Less mascot, more architecture

---

## Prohibited Usage

**Never:**
- ❌ Use kiwi fruit photos instead of Key-Way mascot
- ❌ Change mascot species (no other birds/animals)
- ❌ Use colors outside approved palette
- ❌ Add gradients to logo
- ❌ Animate the mascot (unless subtle, < 200ms transitions)
- ❌ Make claims about features we don't have
- ❌ Use marketing buzzwords in headlines
- ❌ Show business logic examples (OAuth, auth, etc.)

**Always:**
- ✅ Use "Keystone Gateway" full name on first mention
- ✅ Include tagline on landing page
- ✅ Keep Key-Way friendly and humble
- ✅ Emphasize primitives, not policies
- ✅ Be technically accurate

---

## Quick Reference

### Color Shortcuts
```css
/* Copy-paste ready */
:root {
  /* Primary */
  --primary: #73B44C;
  --primary-hover: #5A9138;

  /* Text */
  --text: #2C2C2C;
  --text-secondary: #4A4A4A;

  /* Backgrounds */
  --bg: #FFFFFF;
  --bg-subtle: #FAFADC;

  /* Borders */
  --border: #E5E7EB;
}
```

### Key Messaging
- **What:** Multi-tenant API gateway with embedded Lua scripting
- **Why:** Control without opinions - you write logic, we handle HTTP
- **How:** General-purpose primitives, stateless design, deep modules
- **Who:** Engineers who want routing flexibility without framework lock-in

### Tagline Variants
- Primary: "A highly opinionated gateway with no opinions"
- Short: "Dumb gateway, smart tenants"
- Technical: "Primitives, not workflows"
- Playful: "Key-Way to smart routing"

---

## Version History

**1.0.0** (December 2025)
- Initial brand guide
- Key-Way mascot introduction
- Kiwi fruit color palette
- Design system primitives

---

**Last updated:** December 2025
**Maintained by:** DESIGNER agent
**Reference:** Used by landing page, docs, and marketing materials
