import math
import re
from collections import Counter
from typing import Union, List, Dict, Optional, Tuple, Set
from dataclasses import dataclass
from enum import Enum


class EntropyLevel(Enum):
    LOW = "low"          # < 3.0 бит - слишком предсказуемо
    MEDIUM = "medium"    # 3.0 - 4.5 бит - возможно случайно
    HIGH = "high"        # 4.5 - 5.5 бит - вероятно секрет
    VERY_HIGH = "very_high"  # > 5.5 бит - почти наверняка секрет


@dataclass
class SecretCandidate:
    text: str
    entropy: float
    normalized_entropy: float
    level: EntropyLevel
    alphabet: str
    alphabet_size: int
    length: int
    unique_chars: int
    line_number: int
    file_path: str
    context: str
    confidence: float


class ShannonEntropyAnalyzer: 
    ALPHABETS = {
        'base64': 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=',
        'base64_url': 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_',
        'base32': 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567=',
        'hex': '0123456789abcdefABCDEF',
        'lowercase': 'abcdefghijklmnopqrstuvwxyz',
        'uppercase': 'ABCDEFGHIJKLMNOPQRSTUVWXYZ',
        'digits': '0123456789',
        'alphanumeric': 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789',
        'printable': '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!"#$%&\'()*+,-./:;<=>?@[\\]^_`{|}~ ',
        'binary': '01',
        'hex_full': '0123456789abcdefABCDEF-',
        'uuid': '0123456789abcdefABCDEF-',
        'jwt': 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_',
    }
    
    SECRET_PATTERNS = {
        'api_key': re.compile(r'(?:api[_\s-]?key|apikey|api-token)[\s:=-]+([a-zA-Z0-9_\-]{20,})', re.IGNORECASE),
        'aws_key': re.compile(r'AKIA[0-9A-Z]{16,}', re.IGNORECASE),
        'private_key': re.compile(r'-----BEGIN (?:RSA|DSA|EC|OPENSSH) PRIVATE KEY-----', re.IGNORECASE),
        'password': re.compile(r'(?:password|passwd|pwd)[\s:=-]+([^\s]{8,})', re.IGNORECASE),
        'token': re.compile(r'(?:token|secret|bearer)[\s:=-]+([a-zA-Z0-9_\-\.]{20,})', re.IGNORECASE),
        'jwt': re.compile(r'eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+'),
        'url_secret': re.compile(r'[?&](?:key|token|secret|api_key|apikey)=([a-zA-Z0-9_\-\.]{8,})', re.IGNORECASE),
        'generic_high_entropy': re.compile(r'[\'"`]([a-zA-Z0-9+/=]{20,}[\'"`])'),
    }
    
    def __init__(
        self,
        min_length: int = 8,
        max_length: int = 256,
        entropy_threshold: float = 4.5,
        confidence_threshold: float = 0.6,
        exclude_patterns: Optional[List[str]] = None,
        max_workers: int = 4
    ):
        self.min_length = min_length
        self.max_length = max_length
        self.entropy_threshold = entropy_threshold
        self.confidence_threshold = confidence_threshold
        self.max_workers = max_workers
        
        self._entropy_cache = {}
        
        self.exclude_patterns = []
        if exclude_patterns:
            for pattern in exclude_patterns:
                self.exclude_patterns.append(re.compile(pattern, re.IGNORECASE))
        
        self._init_exclusion_patterns()
    
    def _init_exclusion_patterns(self):
        self.common_false_positives = [
            re.compile(r'^[a-f0-9]{32}$', re.IGNORECASE),  # MD5 hash
            re.compile(r'^[a-f0-9]{40}$', re.IGNORECASE),  # SHA-1
            re.compile(r'^[a-f0-9]{64}$', re.IGNORECASE),  # SHA-256
            re.compile(r'^[a-f0-9]{128}$', re.IGNORECASE), # SHA-512
            re.compile(r'^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$', re.IGNORECASE),  # UUID
            re.compile(r'^[0-9]{6,}$'),
            re.compile(r'^[a-zA-Z]{1,20}$'),
            re.compile(r'^(?:test|example|demo|sample|dummy)[a-z]*$', re.IGNORECASE),
            re.compile(r'^(?:foo|bar|baz|qux)[a-z]*$', re.IGNORECASE),
            re.compile(r'^[a-z]{2,}\.txt$', re.IGNORECASE),
            re.compile(r'^[a-z]{2,}\.log$', re.IGNORECASE),
        ]
    
    def analyze_text(
        self,
        text: str,
        file_path: str = "",
        line_numbers: bool = True
    ) -> List[SecretCandidate]:
        candidates = []
        lines = text.split('\n') if line_numbers else [text]
        
        for idx, line in enumerate(lines, 1):
            for pattern_name, pattern in self.SECRET_PATTERNS.items():
                for match in pattern.finditer(line):
                    secret = match.group(1) if match.groups() else match.group(0)
                    if len(secret) < self.min_length or len(secret) > self.max_length:
                        continue
                    if self._is_false_positive(secret):
                        continue
                    
                    candidate = self._analyze_candidate(
                        secret=secret,
                        line=line,
                        line_number=idx,
                        file_path=file_path,
                        pattern_name=pattern_name
                    )
                    
                    if candidate and candidate.entropy >= self.entropy_threshold:
                        candidates.append(candidate)
        
        return sorted(candidates, key=lambda x: x.confidence, reverse=True)
    
    def _analyze_candidate(
        self,
        secret: str,
        line: str,
        line_number: int,
        file_path: str,
        pattern_name: str
    ) -> Optional[SecretCandidate]:
        best_alphabet = self._determine_best_alphabet(secret)
        
        if not best_alphabet:
            return None
        
        entropy = self._calculate_entropy(secret, best_alphabet)
        
        level = self._get_entropy_level(entropy)
        
        confidence = self._calculate_confidence(
            secret=secret,
            entropy=entropy,
            pattern_name=pattern_name,
            alphabet=best_alphabet
        )
        
        if confidence < self.confidence_threshold:
            return None
        
        return SecretCandidate(
            text=secret,
            entropy=entropy,
            normalized_entropy=entropy / math.log2(len(best_alphabet)) if len(best_alphabet) > 0 else 0,
            level=level,
            alphabet=best_alphabet[:50] + '...' if len(best_alphabet) > 50 else best_alphabet,
            alphabet_size=len(best_alphabet),
            length=len(secret),
            unique_chars=len(set(secret)),
            line_number=line_number,
            file_path=file_path,
            context=self._get_context(line, secret),
            confidence=confidence
        )
    
    def _determine_best_alphabet(self, text: str) -> str:
        text_set = set(text)
        best_alphabet = None
        best_coverage = 0
        
        for name, alphabet in self.ALPHABETS.items():
            alphabet_set = set(alphabet)
            coverage = len(text_set & alphabet_set) / len(text_set) if text_set else 0
            
            if coverage > best_coverage and coverage >= 0.8:
                best_coverage = coverage
                best_alphabet = alphabet
        
        if best_alphabet and best_coverage >= 0.8:
            return best_alphabet
        
        return ''.join(sorted(text_set))
    
    def _calculate_entropy(self, text: str, alphabet: str) -> float:
        cache_key = (text, alphabet)
        if cache_key in self._entropy_cache:
            return self._entropy_cache[cache_key]
        
        valid_chars = [c for c in text if c in alphabet]
        
        if not valid_chars:
            return 0.0
        
        freq = Counter(valid_chars)
        length = len(valid_chars)
        
        entropy = 0.0
        for count in freq.values():
            prob = count / length
            entropy -= prob * math.log2(prob)
        
        self._entropy_cache[cache_key] = entropy
        return entropy
    
    def _get_entropy_level(self, entropy: float) -> EntropyLevel:
        if entropy < 3.0:
            return EntropyLevel.LOW
        elif entropy < 4.5:
            return EntropyLevel.MEDIUM
        elif entropy < 5.5:
            return EntropyLevel.HIGH
        else:
            return EntropyLevel.VERY_HIGH
    
    def _calculate_confidence(
        self,
        secret: str,
        entropy: float,
        pattern_name: str,
        alphabet: str
    ) -> float:
        confidence = 0.0
        
        max_entropy = math.log2(len(alphabet)) if len(alphabet) > 0 else 1
        normalized = entropy / max_entropy if max_entropy > 0 else 0
        confidence += normalized * 0.4
        
        length_factor = min(1.0, (len(secret) - self.min_length) / (self.max_length - self.min_length))
        confidence += length_factor * 0.2
        
        pattern_weights = {
            'private_key': 0.35,
            'aws_key': 0.35,
            'jwt': 0.30,
            'api_key': 0.25,
            'token': 0.20,
            'password': 0.20,
            'url_secret': 0.15,
            'generic_high_entropy': 0.10
        }
        confidence += pattern_weights.get(pattern_name, 0.1)
        
        unique_ratio = len(set(secret)) / len(secret) if len(secret) > 0 else 0
        confidence += unique_ratio * 0.05
        
        return min(confidence, 1.0)
    
    def _is_false_positive(self, text: str) -> bool:
        for pattern in self.common_false_positives:
            if pattern.match(text):
                return True
        
        for pattern in self.exclude_patterns:
            if pattern.search(text):
                return True
        
        if len(set(text)) == 1:
            return True
        
        if text.lower() in ['password', 'secret', 'token', 'key', 'api', 'admin']:
            return True
        
        if max(Counter(text).values()) / len(text) > 0.8:
            return True
        
        return False
    
    def _get_context(self, line: str, secret: str) -> str:
        start = line.find(secret)
        if start == -1:
            return line[:100]
        
        context_start = max(0, start - 50)
        context_end = min(len(line), start + len(secret) + 50)
        
        context = line[context_start:context_end]
        
        return context.replace(secret, '*' * min(len(secret), 10))
    
    def analyze_repository(
        self,
        repo_path: str,
        file_extensions: Optional[List[str]] = None,
        max_file_size: int = 1024 * 1024,  # 1MB
        exclude_dirs: Optional[List[str]] = None
    ) -> List[SecretCandidate]:
        import os
        from pathlib import Path
        
        if exclude_dirs is None:
            exclude_dirs = ['.git', 'node_modules', '__pycache__', '.venv', 'venv', 'env']
        
        if file_extensions is None:
            file_extensions = [
                '.py', '.js', '.ts', '.java', '.cpp', '.c', '.go', '.rb',
                '.php', '.sh', '.bash', '.zsh', '.fish', '.yml', '.yaml',
                '.json', '.xml', '.env', '.txt', '.md', '.conf', '.config',
                '.ini', '.cfg', '.toml', '.properties'
            ]
        
        all_candidates = []
        repo_path = Path(repo_path)
        
        for file_path in repo_path.rglob('*'):
            if file_path.is_dir():
                continue
            
            if any(excluded in str(file_path) for excluded in exclude_dirs):
                continue
            
            if file_extensions and file_path.suffix not in file_extensions:
                continue
            
            if file_path.stat().st_size > max_file_size:
                continue
            
            try:
                with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                
                candidates = self.analyze_text(
                    text=content,
                    file_path=str(file_path)
                )
                
                all_candidates.extend(candidates)
                
            except Exception:
                continue
        
        return sorted(all_candidates, key=lambda x: x.confidence, reverse=True)
    
    def clear_cache(self):
        self._entropy_cache.clear()
    
    def get_stats(self) -> Dict:
        return {
            'min_length': self.min_length,
            'max_length': self.max_length,
            'entropy_threshold': self.entropy_threshold,
            'confidence_threshold': self.confidence_threshold,
            'cache_size': len(self._entropy_cache),
            'patterns_count': len(self.SECRET_PATTERNS),
            'alphabets_count': len(self.ALPHABETS)
        }
