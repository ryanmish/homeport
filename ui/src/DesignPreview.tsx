import { useState } from 'react'
import {
  Github,
  Lock,
  Unlock,
  KeyRound,
  Server,
  GitBranch,
  Copy,
  Share2,
  ChevronRight,
  Home,
  X,
  ExternalLink,
  Terminal
} from 'lucide-react'

// Design Preview Page - Compare different UI variations
export function DesignPreview() {
  const [selectedCard, setSelectedCard] = useState<number>(0)
  const [selectedNav, setSelectedNav] = useState<number>(0)
  const [colorScheme, setColorScheme] = useState<number>(0)

  const cardStyles = [
    {
      name: 'Clean (No shadows)',
      description: 'Flat design with subtle borders only',
      card: 'bg-white border border-gray-200 rounded-lg',
      cardHover: 'hover:border-gray-300',
    },
    {
      name: 'Soft Shadow',
      description: 'Minimal shadow for depth without heavy elevation',
      card: 'bg-white rounded-lg shadow-sm border border-gray-100',
      cardHover: 'hover:shadow-md hover:border-gray-200',
    },
    {
      name: 'Borderless (Shadow only)',
      description: 'No borders, relies entirely on shadow for definition',
      card: 'bg-white rounded-xl shadow-md',
      cardHover: 'hover:shadow-lg',
    },
    {
      name: 'Subtle Fill',
      description: 'Slight gray background, no shadow',
      card: 'bg-gray-50 rounded-lg border border-gray-100',
      cardHover: 'hover:bg-white hover:border-gray-200',
    },
  ]

  const navStyles = [
    {
      name: 'Current (Sticky blur)',
      description: 'Frosted glass effect with blur',
      nav: 'bg-white/95 backdrop-blur-sm border-b border-gray-200',
    },
    {
      name: 'Solid White',
      description: 'Clean solid background',
      nav: 'bg-white border-b border-gray-200 shadow-sm',
    },
    {
      name: 'Dark Header',
      description: 'Dark navigation bar for contrast',
      nav: 'bg-gray-900 text-white',
    },
    {
      name: 'Minimal',
      description: 'No border or shadow, just spacing',
      nav: 'bg-white',
    },
  ]

  const colorSchemes = [
    {
      name: 'Current (Blue primary)',
      primary: 'bg-gray-900 text-white',
      primaryHover: 'hover:bg-gray-800',
      accent: 'text-blue-600',
      badge: 'bg-blue-100 text-blue-800',
    },
    {
      name: 'Blue Primary',
      primary: 'bg-blue-600 text-white',
      primaryHover: 'hover:bg-blue-700',
      accent: 'text-blue-600',
      badge: 'bg-blue-100 text-blue-800',
    },
    {
      name: 'Indigo Primary',
      primary: 'bg-indigo-600 text-white',
      primaryHover: 'hover:bg-indigo-700',
      accent: 'text-indigo-600',
      badge: 'bg-indigo-100 text-indigo-800',
    },
    {
      name: 'Green/Teal',
      primary: 'bg-emerald-600 text-white',
      primaryHover: 'hover:bg-emerald-700',
      accent: 'text-emerald-600',
      badge: 'bg-emerald-100 text-emerald-800',
    },
  ]

  const currentCard = cardStyles[selectedCard]
  const currentNav = navStyles[selectedNav]
  const currentColor = colorSchemes[colorScheme]

  return (
    <div className="min-h-screen bg-gray-100">
      {/* Preview Controls */}
      <div className="sticky top-0 z-50 bg-gray-900 text-white p-4">
        <div className="max-w-6xl mx-auto">
          <div className="flex items-center justify-between mb-4">
            <h1 className="text-xl font-semibold">Design Preview</h1>
            <a
              href="/"
              className="flex items-center gap-2 px-3 py-1.5 bg-white/10 rounded-lg hover:bg-white/20 transition-colors"
            >
              <Home className="h-4 w-4" />
              Back to Dashboard
            </a>
          </div>

          <div className="grid grid-cols-3 gap-6">
            {/* Card Style Selector */}
            <div>
              <label className="block text-sm text-gray-400 mb-2">Card Style</label>
              <div className="flex flex-wrap gap-2">
                {cardStyles.map((style, i) => (
                  <button
                    key={i}
                    onClick={() => setSelectedCard(i)}
                    className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      selectedCard === i
                        ? 'bg-white text-gray-900'
                        : 'bg-white/10 hover:bg-white/20'
                    }`}
                  >
                    {style.name}
                  </button>
                ))}
              </div>
            </div>

            {/* Nav Style Selector */}
            <div>
              <label className="block text-sm text-gray-400 mb-2">Navigation Style</label>
              <div className="flex flex-wrap gap-2">
                {navStyles.map((style, i) => (
                  <button
                    key={i}
                    onClick={() => setSelectedNav(i)}
                    className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      selectedNav === i
                        ? 'bg-white text-gray-900'
                        : 'bg-white/10 hover:bg-white/20'
                    }`}
                  >
                    {style.name}
                  </button>
                ))}
              </div>
            </div>

            {/* Color Scheme Selector */}
            <div>
              <label className="block text-sm text-gray-400 mb-2">Color Scheme</label>
              <div className="flex flex-wrap gap-2">
                {colorSchemes.map((scheme, i) => (
                  <button
                    key={i}
                    onClick={() => setColorScheme(i)}
                    className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      colorScheme === i
                        ? 'bg-white text-gray-900'
                        : 'bg-white/10 hover:bg-white/20'
                    }`}
                  >
                    {scheme.name}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Preview Area */}
      <div className="p-4">
        {/* Navigation Preview */}
        <div className="max-w-6xl mx-auto mb-8">
          <h2 className="text-lg font-semibold text-gray-700 mb-3">Navigation Bar Preview</h2>

          {/* Main Homeport Nav */}
          <div className={`${currentNav.nav} px-6 py-3 rounded-lg mb-4`}>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className={`w-9 h-9 ${currentNav.name === 'Dark Header' ? 'bg-white/20' : 'bg-gray-900'} rounded-xl flex items-center justify-center`}>
                  <Server className={`h-5 w-5 ${currentNav.name === 'Dark Header' ? 'text-white' : 'text-white'}`} />
                </div>
                <span className={`font-semibold text-lg ${currentNav.name === 'Dark Header' ? 'text-white' : 'text-gray-900'}`}>
                  Homeport
                </span>
              </div>

              <div className="flex items-center gap-2">
                <button className={`px-4 py-2 border rounded-lg text-sm font-medium transition-colors ${
                  currentNav.name === 'Dark Header'
                    ? 'border-white/30 text-white hover:bg-white/10'
                    : 'border-gray-300 text-gray-700 hover:bg-gray-50'
                }`}>
                  New
                </button>
                <button className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${currentColor.primary} ${currentColor.primaryHover}`}>
                  Clone
                </button>
              </div>
            </div>
          </div>

          {/* Code-Server Nav with Breadcrumbs */}
          <div className={`${currentNav.nav} px-6 py-3 rounded-lg`}>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className={`w-9 h-9 ${currentNav.name === 'Dark Header' ? 'bg-white/20' : 'bg-gray-900'} rounded-xl flex items-center justify-center`}>
                  <Server className={`h-5 w-5 text-white`} />
                </div>

                {/* Breadcrumb Navigation */}
                <div className={`flex items-center gap-2 text-sm ${currentNav.name === 'Dark Header' ? 'text-white/70' : 'text-gray-500'}`}>
                  <a href="/" className={`${currentNav.name === 'Dark Header' ? 'hover:text-white' : 'hover:text-gray-900'} transition-colors`}>
                    Dashboard
                  </a>
                  <ChevronRight className="h-4 w-4" />
                  <span className={`font-medium ${currentNav.name === 'Dark Header' ? 'text-white' : 'text-gray-900'}`}>
                    my-project
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-2">
                {/* Port indicator with share button */}
                <div className={`flex items-center gap-2 px-3 py-1.5 rounded-lg ${
                  currentNav.name === 'Dark Header' ? 'bg-white/10' : 'bg-gray-100'
                }`}>
                  <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                  <span className={`text-sm font-mono ${currentNav.name === 'Dark Header' ? 'text-white' : 'text-gray-700'}`}>
                    :3000
                  </span>
                  <button className={`p-1 rounded ${currentNav.name === 'Dark Header' ? 'hover:bg-white/10' : 'hover:bg-gray-200'}`}>
                    <Share2 className="h-4 w-4" />
                  </button>
                </div>

                <a
                  href="/"
                  className={`px-4 py-2 border rounded-lg text-sm font-medium transition-colors ${
                    currentNav.name === 'Dark Header'
                      ? 'border-white/30 text-white hover:bg-white/10'
                      : 'border-gray-300 text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  Dashboard
                </a>
              </div>
            </div>
          </div>
        </div>

        {/* Card Styles Preview */}
        <div className="max-w-6xl mx-auto mb-8">
          <h2 className="text-lg font-semibold text-gray-700 mb-3">Repository Card Preview</h2>

          <div className={`${currentCard.card} ${currentCard.cardHover} p-5 transition-all`}>
            {/* Card Header */}
            <div className="flex items-start justify-between mb-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-gray-100 rounded-lg">
                  <GitBranch className="h-5 w-5 text-gray-600" />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <h3 className="font-semibold text-gray-900">my-awesome-project</h3>
                    <span className="px-2 py-0.5 bg-amber-100 text-amber-800 text-xs font-medium rounded-full">
                      Needs install
                    </span>
                  </div>
                  <div className="flex items-center gap-2 text-sm text-gray-500">
                    <span className="flex items-center gap-1">
                      <GitBranch className="h-3.5 w-3.5" />
                      main
                    </span>
                    <span className="text-gray-300">|</span>
                    <span className="flex items-center gap-1 text-green-600">
                      <ArrowUp className="h-3.5 w-3.5" />
                      2 ahead
                    </span>
                  </div>
                </div>
              </div>

              <a
                href="#"
                className="p-2 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors"
              >
                <Github className="h-5 w-5" />
              </a>
            </div>

            {/* Port Row */}
            <div className="p-3 bg-gray-50 rounded-lg flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                <code className="px-2 py-1 bg-white border border-gray-200 rounded text-sm font-mono">
                  :3000
                </code>
                <span className="text-sm text-gray-600">next-server</span>
                <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${currentColor.badge}`}>
                  <Lock className="h-3 w-3 inline mr-1" />
                  Private
                </span>
              </div>

              <div className="flex items-center gap-1">
                <button className="p-2 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors">
                  <Copy className="h-4 w-4" />
                </button>
                <button className="p-2 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors">
                  <ExternalLink className="h-4 w-4" />
                </button>
                <button className="p-2 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors">
                  <Share2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* Button Styles */}
        <div className="max-w-6xl mx-auto mb-8">
          <h2 className="text-lg font-semibold text-gray-700 mb-3">Button Styles</h2>

          <div className={`${currentCard.card} p-5`}>
            <div className="flex flex-wrap gap-4">
              {/* Primary */}
              <button className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${currentColor.primary} ${currentColor.primaryHover}`}>
                Primary Action
              </button>

              {/* GitHub Style (User's favorite) */}
              <button className="px-4 py-2 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors flex items-center gap-2">
                <Github className="h-4 w-4" />
                GitHub Style
              </button>

              {/* Outline */}
              <button className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors">
                Outline
              </button>

              {/* Ghost */}
              <button className="px-4 py-2 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-100 transition-colors">
                Ghost
              </button>

              {/* Destructive */}
              <button className="px-4 py-2 bg-red-600 text-white rounded-lg text-sm font-medium hover:bg-red-700 transition-colors">
                Destructive
              </button>

              {/* Icon Button */}
              <button className="p-2 border border-gray-300 text-gray-700 rounded-lg hover:bg-gray-50 transition-colors">
                <Terminal className="h-5 w-5" />
              </button>
            </div>
          </div>
        </div>

        {/* Share Modal Preview */}
        <div className="max-w-6xl mx-auto mb-8">
          <h2 className="text-lg font-semibold text-gray-700 mb-3">Share Modal Preview</h2>

          <div className={`${currentCard.card} max-w-md p-6`}>
            <div className="flex items-center justify-between mb-6">
              <h3 className="text-lg font-semibold text-gray-900">Share Port :3000</h3>
              <button className="p-1 text-gray-400 hover:text-gray-600">
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="space-y-3">
              {[
                { icon: Lock, label: 'Private', desc: 'Only you can access', active: true },
                { icon: KeyRound, label: 'Password', desc: 'Anyone with the password', active: false },
                { icon: Unlock, label: 'Public', desc: 'Anyone with the link', active: false },
              ].map((option, i) => (
                <button
                  key={i}
                  className={`w-full p-4 rounded-lg border-2 text-left transition-colors ${
                    option.active
                      ? `border-blue-500 bg-blue-50`
                      : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <option.icon className={`h-5 w-5 ${option.active ? 'text-blue-600' : 'text-gray-500'}`} />
                    <div>
                      <div className={`font-medium ${option.active ? 'text-blue-900' : 'text-gray-900'}`}>
                        {option.label}
                      </div>
                      <div className={`text-sm ${option.active ? 'text-blue-700' : 'text-gray-500'}`}>
                        {option.desc}
                      </div>
                    </div>
                  </div>
                </button>
              ))}
            </div>

            <div className="flex justify-end gap-3 mt-6">
              <button className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg text-sm font-medium transition-colors">
                Cancel
              </button>
              <button className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${currentColor.primary} ${currentColor.primaryHover}`}>
                Apply
              </button>
            </div>
          </div>
        </div>

        {/* Stats Cards Preview */}
        <div className="max-w-6xl mx-auto mb-8">
          <h2 className="text-lg font-semibold text-gray-700 mb-3">Stats Cards Preview</h2>

          <div className="grid grid-cols-3 gap-4">
            {[
              { label: 'CPU', value: '23%', color: 'bg-green-500' },
              { label: 'Memory', value: '67%', color: 'bg-yellow-500' },
              { label: 'Disk', value: '45%', color: 'bg-green-500' },
            ].map((stat, i) => (
              <div key={i} className={`${currentCard.card} ${currentCard.cardHover} p-4 transition-all`}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm text-gray-500">{stat.label}</span>
                  <span className="text-lg font-semibold text-gray-900">{stat.value}</span>
                </div>
                <div className="h-1.5 bg-gray-200 rounded-full overflow-hidden">
                  <div
                    className={`h-full ${stat.color} rounded-full`}
                    style={{ width: stat.value }}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Badge Variants */}
        <div className="max-w-6xl mx-auto mb-8">
          <h2 className="text-lg font-semibold text-gray-700 mb-3">Badge Variants</h2>

          <div className={`${currentCard.card} p-5`}>
            <div className="flex flex-wrap gap-3">
              <span className={`px-2.5 py-0.5 text-xs font-medium rounded-full ${currentColor.badge}`}>
                <Lock className="h-3 w-3 inline mr-1" />
                Private
              </span>
              <span className="px-2.5 py-0.5 bg-amber-100 text-amber-800 text-xs font-medium rounded-full">
                <KeyRound className="h-3 w-3 inline mr-1" />
                Password
              </span>
              <span className="px-2.5 py-0.5 bg-green-100 text-green-800 text-xs font-medium rounded-full">
                <Unlock className="h-3 w-3 inline mr-1" />
                Public
              </span>
              <span className="px-2.5 py-0.5 bg-amber-100 text-amber-800 text-xs font-medium rounded-full">
                Needs install
              </span>
              <span className="px-2.5 py-0.5 bg-gray-100 text-gray-700 text-xs font-medium rounded-full">
                External
              </span>
            </div>
          </div>
        </div>

        {/* Current Selection Summary */}
        <div className="max-w-6xl mx-auto">
          <div className="bg-gray-900 text-white p-6 rounded-lg">
            <h2 className="text-lg font-semibold mb-4">Your Current Selection</h2>
            <div className="grid grid-cols-3 gap-6 text-sm">
              <div>
                <span className="text-gray-400">Card Style:</span>
                <p className="font-medium">{currentCard.name}</p>
                <p className="text-gray-400 text-xs mt-1">{currentCard.description}</p>
              </div>
              <div>
                <span className="text-gray-400">Navigation:</span>
                <p className="font-medium">{currentNav.name}</p>
                <p className="text-gray-400 text-xs mt-1">{currentNav.description}</p>
              </div>
              <div>
                <span className="text-gray-400">Colors:</span>
                <p className="font-medium">{currentColor.name}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

// Missing import fix
function ArrowUp(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}>
      <path d="m5 12 7-7 7 7"/>
      <path d="M12 19V5"/>
    </svg>
  )
}
