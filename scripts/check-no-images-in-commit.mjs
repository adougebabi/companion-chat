import {spawnSync} from 'node:child_process';
import {extname} from 'node:path';

const imageExtensions=new Set(['.png','.apng','.jpg','.jpeg','.jfif','.gif','.webp','.avif','.bmp','.tif','.tiff','.ico','.icns','.svg','.svgz','.heic','.heif','.psd','.xcf','.tga','.dds','.exr','.hdr']);
const diff=spawnSync('git',['diff','--cached','--name-only','--diff-filter=ACMR','-z'],{encoding:'utf8'});
if(diff.error||diff.status!==0){
  process.stderr.write('无法检查暂存区图片，提交已停止。\n'+(diff.error?.message||diff.stderr||''));
  process.exit(1);
}
const images=diff.stdout.split('\0').filter(path=>path&&imageExtensions.has(extname(path).toLowerCase()));
if(images.length){
  process.stderr.write('禁止提交图片文件。请从暂存区移除图片，只提交代码和文字报告；删除图片允许提交。\n');
  for(const path of images)process.stderr.write('  '+JSON.stringify(path)+'\n');
  process.exit(1);
}
