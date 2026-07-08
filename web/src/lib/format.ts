// Ortak biçimlendirme — slug→müdür-dili ad, tool→yetenek etiketi. Wizard + pano paylaşır.

// slug → insan-dostu ad ("oturum_katilimlari" → "Oturum Katilimlari").
export function humanize(s: string): string {
  return s
    .replace(/[_.]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

// tool adı → yetenek etiketi + örnek soru (count_/list_ deseni; müdür dili).
export function capability(toolName: string, friendly: string): { label: string; example: string } {
  const low = friendly.toLowerCase()
  if (toolName.startsWith('count_')) return { label: `${friendly} sayısını söyleyebilir`, example: `Kaç ${low} var?` }
  if (toolName.startsWith('list_')) return { label: `${friendly} listeleyebilir`, example: `${friendly} göster` }
  return { label: toolName, example: '' }
}

const kindLabels: Record<string, string> = { mssql: 'SQL Server', erpnext: 'ErpNext' }
export function kindLabel(kind: string): string {
  return kindLabels[kind] ?? kind
}
