import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "POS",
    short_name: "POS",
    description: "Local-first point of sale app",
    start_url: "/",
    display: "standalone",
    background_color: "#0b160a",
    theme_color: "#14591d",
    icons: [
      {
        src: "/icons/icon-192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/icons/icon-512.png",
        sizes: "512x512",
        type: "image/png",
      },
    ],
  };
}
