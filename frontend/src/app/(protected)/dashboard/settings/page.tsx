"use client"

import { useAuth } from "@/providers/auth-provider"
import { useTenant } from "@/contexts/tenant-context"
import { useListTenantMembersQuery } from "@/store/services"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { motion } from "framer-motion"
import { Shield, Bell, Palette, Lock, Monitor, Sun, Moon, Users, Mail, Plus, X } from "lucide-react"
import { useState, useEffect } from "react"
import { useTheme } from "next-themes"

export default function SettingsPage() {
  const { user } = useAuth()
  const { tenantId } = useTenant()
  const { theme, setTheme } = useTheme()
  const [notifications, setNotifications] = useState({
    emailAlerts: true,
    weeklyReport: true,
    incidentNotifications: true,
  })
  const [showInviteModal, setShowInviteModal] = useState(false)
  const [inviteEmail, setInviteEmail] = useState("")
  const [inviteRole, setInviteRole] = useState("user")
  const [inviteToast, setInviteToast] = useState(false)

  useEffect(() => {
    if (inviteToast) {
      const t = setTimeout(() => setInviteToast(false), 3000)
      return () => clearTimeout(t)
    }
  }, [inviteToast])

  const { data: membersData, error: membersError } = useListTenantMembersQuery(tenantId ?? "", {
    skip: !tenantId,
  })

  const handleInvite = () => {
    if (!inviteEmail.trim()) return
    setShowInviteModal(false)
    setInviteEmail("")
    setInviteRole("user")
    setInviteToast(true)
  }

  // Determine current user's role from members list
  const currentUserMember = membersData?.members?.find((m) => m.email === user?.email)
  const is404 = membersError && (membersError as { status?: number }).status === 404

  return (
    <div>
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
        className="mb-8"
      >
        <h1 className="text-4xl font-bold text-slate-900 dark:text-white mb-2">
          Settings
        </h1>
        <p className="text-slate-600 dark:text-slate-400">
          Manage your profile, preferences, and security
        </p>
      </motion.div>

      {/* Settings Sections */}
      <div className="space-y-6">
        {/* Profile Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.1 }}
        >
          <Card className="p-6">
            <div className="flex items-center gap-3 mb-6">
              <Shield className="w-5 h-5 text-slate-600 dark:text-slate-400" />
              <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
                Profile
              </h2>
            </div>
            <div className="space-y-4">
              <div className="grid md:grid-cols-2 gap-4">
                <div>
                  <Label htmlFor="firstName">First Name</Label>
                  <Input
                    id="firstName"
                    value={user?.firstName || ""}
                    disabled
                    className="mt-2"
                  />
                </div>
                <div>
                  <Label htmlFor="lastName">Last Name</Label>
                  <Input
                    id="lastName"
                    value={user?.lastName || ""}
                    disabled
                    className="mt-2"
                  />
                </div>
              </div>
              <div>
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  value={user?.email || ""}
                  disabled
                  className="mt-2"
                />
              </div>
              <Button disabled>Update Profile</Button>
            </div>
          </Card>
        </motion.div>

        {/* Notifications Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.2 }}
        >
          <Card className="p-6">
            <div className="flex items-center gap-3 mb-6">
              <Bell className="w-5 h-5 text-slate-600 dark:text-slate-400" />
              <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
                Notifications
              </h2>
            </div>
            <div className="space-y-4">
              {Object.entries(notifications).map(([key, value]) => (
                <label key={key} className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={value}
                    onChange={(e) =>
                      setNotifications({
                        ...notifications,
                        [key]: e.target.checked,
                      })
                    }
                    className="w-4 h-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                  />
                  <span className="text-slate-700 dark:text-slate-300 capitalize">
                    {key.replace(/([A-Z])/g, " $1").trim()}
                  </span>
                </label>
              ))}
            </div>
            <Button className="mt-6">Save Preferences</Button>
          </Card>
        </motion.div>

        {/* Security Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.3 }}
        >
          <Card className="p-6">
            <div className="flex items-center gap-3 mb-6">
              <Lock className="w-5 h-5 text-slate-600 dark:text-slate-400" />
              <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
                Security
              </h2>
            </div>
            <div className="space-y-4">
              <div>
                <p className="text-slate-700 dark:text-slate-300 mb-4">
                  Your password is managed through Keycloak. To change your password or enable two-factor authentication, please visit your account settings in Keycloak.
                </p>
                <Button variant="outline">
                  Open Keycloak Account Settings
                </Button>
              </div>
            </div>
          </Card>
        </motion.div>

        {/* Appearance Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.4 }}
        >
          <Card className="p-6">
            <div className="flex items-center gap-3 mb-6">
              <Palette className="w-5 h-5 text-slate-600 dark:text-slate-400" />
              <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
                Appearance
              </h2>
            </div>
            <div className="grid grid-cols-3 gap-3">
              {[
                { value: "light", label: "Light", icon: <Sun className="w-5 h-5" /> },
                { value: "dark", label: "Dark", icon: <Moon className="w-5 h-5" /> },
                { value: "system", label: "System", icon: <Monitor className="w-5 h-5" /> },
              ].map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setTheme(opt.value)}
                  className={`flex flex-col items-center gap-2 p-4 rounded-xl border-2 transition-all ${
                    theme === opt.value
                      ? "border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300"
                      : "border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-slate-300 dark:hover:border-slate-600"
                  }`}
                >
                  {opt.icon}
                  <span className="text-sm font-medium">{opt.label}</span>
                </button>
              ))}
            </div>
          </Card>
        </motion.div>

        {/* Team Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.5 }}
        >
          <Card className="p-6">
            <div className="flex items-center justify-between mb-6">
              <div className="flex items-center gap-3">
                <Users className="w-5 h-5 text-slate-600 dark:text-slate-400" />
                <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
                  Team
                </h2>
                {currentUserMember && (
                  <Badge variant="secondary" className="text-xs">
                    Your role: {currentUserMember.role}
                  </Badge>
                )}
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setShowInviteModal(true)}
              >
                <Plus className="w-3.5 h-3.5 mr-1.5" />
                Invite Member
              </Button>
            </div>

            {is404 ? (
              <p className="text-sm text-slate-500 dark:text-slate-400">
                Member management requires admin configuration.
              </p>
            ) : !membersData ? (
              <p className="text-sm text-slate-400 animate-pulse">Loading members…</p>
            ) : membersData.members.length === 0 ? (
              <p className="text-sm text-slate-500 dark:text-slate-400">
                No team members yet. Invite someone to get started.
              </p>
            ) : (
              <div className="space-y-2">
                {membersData.members.map((member) => (
                  <div
                    key={member.email}
                    className="flex items-center justify-between p-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900"
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-xs font-bold text-blue-700 dark:text-blue-300">
                        {member.email[0]?.toUpperCase()}
                      </div>
                      <div>
                        <p className="text-sm font-medium text-slate-800 dark:text-slate-100">{member.email}</p>
                        <p className="text-xs text-slate-400">
                          Joined {new Date(member.joinedAt).toLocaleDateString()}
                        </p>
                      </div>
                    </div>
                    <Badge variant="secondary" className="text-xs capitalize">{member.role}</Badge>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </motion.div>
      </div>

      {/* Invite toast */}
      {inviteToast && (
        <div className="fixed bottom-6 right-6 z-50 px-4 py-3 rounded-xl bg-green-600 text-white text-sm font-medium shadow-lg flex items-center gap-2">
          <Mail className="w-4 h-4" />
          Invitation sent
        </div>
      )}

      {/* Invite member modal */}
      {showInviteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
          <div className="w-full max-w-sm bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-700 p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-bold text-slate-900 dark:text-white">Invite Member</h3>
              <button onClick={() => setShowInviteModal(false)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-3">
              <div>
                <Label htmlFor="inviteEmail">Email address</Label>
                <Input
                  id="inviteEmail"
                  type="email"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder="colleague@company.com"
                  className="mt-1"
                />
              </div>
              <div>
                <Label htmlFor="inviteRole">Role</Label>
                <select
                  id="inviteRole"
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value)}
                  className="mt-1 w-full text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-slate-700 dark:text-slate-300"
                >
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                  <option value="analyst">Analyst</option>
                  <option value="viewer">Viewer</option>
                </select>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <Button variant="outline" className="flex-1" onClick={() => setShowInviteModal(false)}>
                Cancel
              </Button>
              <Button className="flex-1" onClick={handleInvite} disabled={!inviteEmail.trim()}>
                Send Invitation
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
