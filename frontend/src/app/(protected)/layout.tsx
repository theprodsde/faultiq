"use client"

import { DashboardLayout } from "@/components/layouts/DashboardLayout"
import { ProtectedRoute } from "@/components/ProtectedRoute"

export default function DashboardLayoutWrapper({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <ProtectedRoute>
      <DashboardLayout>{children}</DashboardLayout>
    </ProtectedRoute>
  )
}
