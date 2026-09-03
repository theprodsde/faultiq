"use client"

import { useEffect, useState } from "react"

interface Props {
  iso?: string | number | Date | null
  options?: Intl.DateTimeFormatOptions
}

export default function ClientDate({ iso, options }: Props) {
  const [label, setLabel] = useState("")

  useEffect(() => {
    if (!iso) {
      setLabel("")
      return
    }
    const d = typeof iso === "string" || typeof iso === "number" ? new Date(iso) : iso
    try {
      const locale = typeof navigator !== "undefined" ? navigator.language : "en-US"
      setLabel(d.toLocaleString(locale, options))
    } catch (e) {
      setLabel(d.toString())
    }
  }, [iso, options])

  return <>{label}</>
}
