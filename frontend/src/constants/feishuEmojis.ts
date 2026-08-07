export interface FeishuEmoji {
  code: string
  name: string
  keywords: string[]
  imageUrl: string
}

const emojiImages = import.meta.glob('../assets/feishu-emojis/*.png', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>

function emoji(code: string, name: string, keywords: string[] = []): FeishuEmoji {
  const imagePath = `../assets/feishu-emojis/${code.toLowerCase()}.png`
  return {
    code,
    name,
    keywords,
    imageUrl: emojiImages[imagePath] || '',
  }
}

export const feishuEmojis: FeishuEmoji[] = [
  emoji('OK', '好的', ['确认', '可以']),
  emoji('THUMBSUP', '点赞', ['赞', '同意']),
  emoji('THANKS', '谢谢', ['感谢']),
  emoji('MUSCLE', '加油', ['努力', '力量']),
  emoji('FINGERHEART', '比心', ['爱心']),
  emoji('APPLAUSE', '鼓掌', ['掌声']),
  emoji('FISTBUMP', '碰拳', ['合作']),
  emoji('JIAYI', '加一', ['赞同']),
  emoji('DONE', '完成', ['搞定']),
  emoji('SMILE', '微笑', ['开心']),
  emoji('BLUSH', '害羞笑', ['开心']),
  emoji('LAUGH', '大笑', ['开心']),
  emoji('LOL', '笑哭', ['大笑']),
  emoji('FACEPALM', '捂脸', ['无奈']),
  emoji('LOVE', '喜欢', ['爱']),
  emoji('WINK', '眨眼', ['俏皮']),
  emoji('PROUD', '得意', ['骄傲']),
  emoji('THINKING', '思考', ['考虑']),
  emoji('SOB', '流泪', ['难过']),
  emoji('CRY', '哭泣', ['伤心']),
  emoji('JOYFUL', '愉快', ['开心']),
  emoji('WOW', '惊叹', ['哇']),
  emoji('YEAH', '耶', ['胜利']),
  emoji('EMBARRASSED', '尴尬', ['不好意思']),
  emoji('KISS', '亲亲', ['喜欢']),
  emoji('CLAP', '拍手', ['鼓掌']),
  emoji('PRAISE', '称赞', ['表扬']),
  emoji('COMFORT', '安慰', ['关心']),
  emoji('HUG', '拥抱', ['关心']),
  emoji('WAVE', '挥手', ['你好', '再见']),
  emoji('ANGRY', '生气', ['愤怒']),
  emoji('SWEAT', '流汗', ['紧张']),
  emoji('LGTM', '看起来不错', ['通过']),
  emoji('SALUTE', '敬礼', ['收到']),
  emoji('HIGHFIVE', '击掌', ['庆祝']),
  emoji('ROSE', '玫瑰', ['鲜花']),
  emoji('HEART', '爱心', ['喜欢']),
  emoji('PARTY', '庆祝', ['派对']),
  emoji('CAKE', '蛋糕', ['生日']),
  emoji('GIFT', '礼物', ['祝福']),
]

export const feishuEmojiByCode = new Map(feishuEmojis.map((item) => [item.code, item]))

export function filterFeishuEmojis(query: string) {
  const normalized = query.trim().toLocaleLowerCase()
  if (!normalized) return feishuEmojis
  return feishuEmojis.filter((item) => [item.code, item.name, ...item.keywords]
    .some((value) => value.toLocaleLowerCase().includes(normalized)))
}
