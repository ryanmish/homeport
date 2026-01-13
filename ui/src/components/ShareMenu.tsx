import { useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { Button } from '@/components/ui/button'

export interface ShareMenuPort {
  port: number
  share_mode: 'private' | 'password' | 'public'
}

export type ShareMenuTheme = 'light' | 'dark'

export interface ShareMenuProps {
  port: ShareMenuPort
  theme: ShareMenuTheme
  onShare: (mode: string, password?: string, expiresIn?: string) => void
  onClose: () => void
  onCopyUrl: () => void
}

export function ShareMenu({
  port,
  theme,
  onShare,
  onClose,
  onCopyUrl
}: ShareMenuProps) {
  const [mode, setMode] = useState(port.share_mode)
  // Load saved password from localStorage
  const [password, setPassword] = useState(() => {
    return localStorage.getItem(`homeport_share_pw_${port.port}`) || ''
  })
  const [showPassword, setShowPassword] = useState(true) // Show password by default

  const isValid = mode !== 'password' || password.length > 0

  const handleApply = (shouldCopy: boolean) => {
    // Save password to localStorage so user can see it later
    if (mode === 'password' && password) {
      localStorage.setItem(`homeport_share_pw_${port.port}`, password)
    } else {
      // Clear saved password if switching away from password mode
      localStorage.removeItem(`homeport_share_pw_${port.port}`)
    }
    onShare(mode, mode === 'password' ? password : undefined, undefined)
    if (shouldCopy) {
      onCopyUrl()
    }
  }

  const OptionButton = ({ value, title, desc }: { value: string; title: string; desc: string }) => (
    <button
      onClick={() => setMode(value as typeof mode)}
      className={`w-full p-3 rounded-lg border text-left transition-colors ${
        mode === value
          ? theme === 'dark' ? 'border-blue-500 bg-blue-500/10' : 'border-blue-500 bg-blue-50'
          : theme === 'dark' ? 'border-gray-700 hover:border-gray-600' : 'border-gray-200 hover:border-gray-300'
      }`}
    >
      <div className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>{title}</div>
      <div className={`text-xs mt-0.5 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>{desc}</div>
    </button>
  )

  return (
    <div className={`absolute right-0 top-full mt-1 w-80 rounded-xl shadow-lg border p-4 z-50 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
      <div className="space-y-4">
        <div className={`text-sm font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Share Settings</div>

        {/* Mode options with password field inline */}
        <div className="space-y-2">
          <OptionButton value="private" title="Private" desc="Only accessible when logged into Homeport" />
          <OptionButton value="password" title="Password" desc="Anyone with the password can access" />

          {/* Password field - directly under Password option */}
          {mode === 'password' && (
            <div className="relative -mt-1">
              <input
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter password..."
                className={`w-full px-3 py-2 pr-10 text-sm border rounded-lg focus:outline-none focus:ring-2 ${theme === 'dark' ? 'bg-gray-900 border-gray-600 text-gray-100 focus:ring-gray-500' : 'border-gray-200 focus:ring-gray-400'}`}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className={`absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded ${theme === 'dark' ? 'text-gray-500 hover:text-gray-300' : 'text-gray-400 hover:text-gray-600'}`}
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          )}

          <OptionButton value="public" title="Public" desc="Anyone with the link can access" />
        </div>

        {/* Buttons */}
        <div className={`flex justify-end gap-2 pt-3 border-t ${theme === 'dark' ? 'border-gray-700' : 'border-gray-100'}`}>
          <Button variant="outline" size="sm" onClick={onClose} className="text-xs">
            Cancel
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleApply(true)}
            disabled={!isValid}
            className="text-xs"
          >
            Apply & Copy URL
          </Button>
          <Button
            size="sm"
            onClick={() => handleApply(false)}
            disabled={!isValid}
            className="text-xs"
          >
            Apply
          </Button>
        </div>
      </div>
    </div>
  )
}

export default ShareMenu
