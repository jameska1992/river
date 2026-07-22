import { RiFoldersLine } from 'react-icons/ri'
import { imageUrl } from '../util/imageUrl'
import styles from './CollectionPreview.module.css'

interface Props {
  covers: string[]
  name: string
  className?: string
  emptyIconSize?: number
}

// Shared preview art used by the home-page collections carousel and the
// collection detail page header so both surfaces show identical artwork
// for a given collection: nothing → folder icon, 1–3 covers → single, 4+ →
// 2×2 collage.
export function CollectionPreview({ covers, name, className, emptyIconSize = 32 }: Props) {
  return (
    <div className={`${styles.art} ${className ?? ''}`}>
      {covers.length === 0 ? (
        <div className={styles.empty}>
          <RiFoldersLine size={emptyIconSize} className={styles.emptyIcon} />
        </div>
      ) : covers.length < 4 ? (
        <img src={imageUrl(covers[0])} alt={name} className={styles.single} />
      ) : (
        <div className={styles.collage}>
          {covers.slice(0, 4).map((src, i) => (
            <img key={i} src={imageUrl(src)} alt="" className={styles.collageImg} />
          ))}
        </div>
      )}
    </div>
  )
}
