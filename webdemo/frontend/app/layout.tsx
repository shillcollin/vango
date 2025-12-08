import type { Metadata } from "next"
import { Hind } from "next/font/google"
import "./globals.css"

const hind = Hind({
  subsets: ["latin"],
  weight: ["300", "400", "500", "600", "700"],
  variable: "--font-hind",
})

export const metadata: Metadata = {
  title: "GAI Web Demo",
  description: "Explore OpenAI, Anthropic, and Gemini through the GAI SDK",
}

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={hind.variable}>
      <body className={hind.className}>{children}</body>
    </html>
  )
}
