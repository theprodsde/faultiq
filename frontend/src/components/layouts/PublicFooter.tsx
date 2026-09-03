import Link from "next/link"

export function PublicFooter() {
  return (
    <footer className="border-t border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-950">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="grid md:grid-cols-4 gap-8 mb-8">
          {/* Brand */}
          <div>
            <div className="text-xl font-bold bg-gradient-to-r from-blue-600 to-cyan-600 bg-clip-text text-transparent mb-2">
              ⚡ FaultIQ
            </div>
            <p className="text-sm text-slate-600 dark:text-slate-400">
              Enterprise API Intelligence Platform for incident detection and system reliability.
            </p>
          </div>

          {/* Product */}
          <div>
            <h3 className="font-semibold text-slate-900 dark:text-slate-50 mb-4">
              Product
            </h3>
            <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-400">
              <li>
                <Link href="/features" className="hover:text-slate-900 dark:hover:text-slate-50">
                  Features
                </Link>
              </li>
              <li>
                <Link href="/pricing" className="hover:text-slate-900 dark:hover:text-slate-50">
                  Pricing
                </Link>
              </li>
              <li>
                <Link href="/docs" className="hover:text-slate-900 dark:hover:text-slate-50">
                  Documentation
                </Link>
              </li>
            </ul>
          </div>

          {/* Company */}
          <div>
            <h3 className="font-semibold text-slate-900 dark:text-slate-50 mb-4">
              Company
            </h3>
            <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-400">
              <li>
                <Link href="/about" className="hover:text-slate-900 dark:hover:text-slate-50">
                  About
                </Link>
              </li>
              <li>
                <Link href="/blog" className="hover:text-slate-900 dark:hover:text-slate-50">
                  Blog
                </Link>
              </li>
              <li>
                <Link href="/contact" className="hover:text-slate-900 dark:hover:text-slate-50">
                  Contact
                </Link>
              </li>
            </ul>
          </div>

          {/* Legal */}
          <div>
            <h3 className="font-semibold text-slate-900 dark:text-slate-50 mb-4">
              Legal
            </h3>
            <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-400">
              <li>
                <Link href="/privacy" className="hover:text-slate-900 dark:hover:text-slate-50">
                  Privacy Policy
                </Link>
              </li>
              <li>
                <Link href="/terms" className="hover:text-slate-900 dark:hover:text-slate-50">
                  Terms of Service
                </Link>
              </li>
            </ul>
          </div>
        </div>

        <div className="border-t border-slate-200 dark:border-slate-800 pt-8">
          <p className="text-center text-sm text-slate-600 dark:text-slate-400">
            © {new Date().getFullYear()} FaultIQ. All rights reserved.
          </p>
        </div>
      </div>
    </footer>
  )
}
