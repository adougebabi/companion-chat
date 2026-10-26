import { acceptancePath } from './v3/mock-navigation.mjs';
import { createRoot } from 'react-dom/client';
import App from './react-app/App';
import './styles/react.css';
const bookmarkedPage=acceptancePath(location.pathname,location.search);
if(!location.hash&&bookmarkedPage)history.replaceState(null,'','/#'+bookmarkedPage);
const host=document.getElementById('app');
if(!host)throw new Error('Product root is missing');
createRoot(host).render(<App/>);
